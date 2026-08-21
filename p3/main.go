package main

import (
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

// ==========================================
// 1. CONFIGURATION & DOMAIN
// ==========================================

const (
	defaultListenAddr = "127.0.0.1:1053" // Menggunakan 1053 (Bebas dari mDNS / Avahi)
	defaultUpstreams  = "1.1.1.1:53,8.8.8.8:53"
	shardCount        = 64
	udpBufferSize     = 4096
)

type CacheItem struct {
	payload   []byte
	expiresAt time.Time
}

type CacheEntry struct {
	RawPayload []byte
	ExpiresAt  time.Time
	IsStale    bool
}

// ==========================================
// 2. NATIVE SOCKS 4 & SOCKS 5 DIALER ENGINE
// ==========================================

// SOCKSClient menangani koneksi tunneling RFC 1928 (SOCKS5) dan SOCKS4
type SOCKSClient struct {
	ProxyAddr string
	Version   int // 4 atau 5
	Timeout   time.Duration
}

func (s *SOCKSClient) Dial(ctx context.Context, targetAddr string) (net.Conn, error) {
	d := net.Dialer{Timeout: s.Timeout}
	conn, err := d.DialContext(ctx, "tcp", s.ProxyAddr)
	if err != nil {
		return nil, fmt.Errorf("gagal koneksi ke SOCKS server (%s): %w", s.ProxyAddr, err)
	}

	_ = conn.SetDeadline(time.Now().Add(s.Timeout))

	host, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		conn.Close()
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		conn.Close()
		return nil, err
	}

	if s.Version == 4 {
		err = s.handshakeSOCKS4(conn, host, uint16(port))
	} else {
		err = s.handshakeSOCKS5(conn, host, uint16(port))
	}

	if err != nil {
		conn.Close()
		return nil, err
	}

	// Reset deadline setelah handshake sukses
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func (s *SOCKSClient) handshakeSOCKS5(conn net.Conn, host string, port uint16) error {
	// 1. Version Identifier & Method Selection (Tanpa Otentikasi: 0x00)
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return err
	}

	var methodResp [2]byte
	if _, err := io.ReadFull(conn, methodResp[:]); err != nil {
		return err
	}
	if methodResp[0] != 0x05 || methodResp[1] != 0x00 {
		return fmt.Errorf("SOCKS5 menolak metode otentikasi: ver=%d, method=%d", methodResp[0], methodResp[1])
	}

	// 2. Request Details (CMD 0x01 = CONNECT)
	req := []byte{0x05, 0x01, 0x00}
	ip := net.ParseIP(host)
	if ip4 := ip.To4(); ip4 != nil {
		req = append(req, 0x01) // IPv4
		req = append(req, ip4...)
	} else {
		req = append(req, 0x03) // Domain name
		req = append(req, byte(len(host)))
		req = append(req, []byte(host)...)
	}

	var portBuf [2]byte
	binary.BigEndian.PutUint16(portBuf[:], port)
	req = append(req, portBuf[:]...)

	if _, err := conn.Write(req); err != nil {
		return err
	}

	// 3. Response Evaluation
	var reply [4]byte
	if _, err := io.ReadFull(conn, reply[:]); err != nil {
		return err
	}
	if reply[1] != 0x00 {
		return fmt.Errorf("SOCKS5 connect gagal dengan status code: 0x%02X", reply[1])
	}

	// Discard bound address
	switch reply[3] {
	case 0x01: // IPv4
		var discard [4 + 2]byte
		_, _ = io.ReadFull(conn, discard[:])
	case 0x03: // Domain
		var l [1]byte
		_, _ = io.ReadFull(conn, l[:])
		buf := make([]byte, int(l[0])+2)
		_, _ = io.ReadFull(conn, buf)
	case 0x04: // IPv6
		var discard [16 + 2]byte
		_, _ = io.ReadFull(conn, discard[:])
	}

	return nil
}

func (s *SOCKSClient) handshakeSOCKS4(conn net.Conn, host string, port uint16) error {
	ip := net.ParseIP(host).To4()
	if ip == nil {
		return errors.New("SOCKS4 hanya mendukung target IPv4")
	}

	req := make([]byte, 9)
	req[0] = 0x04 // Version 4
	req[1] = 0x01 // CMD CONNECT
	binary.BigEndian.PutUint16(req[2:4], port)
	copy(req[4:8], ip)
	req[8] = 0x00 // Null string terminator for User ID

	if _, err := conn.Write(req); err != nil {
		return err
	}

	var resp [8]byte
	if _, err := io.ReadFull(conn, resp[:]); err != nil {
		return err
	}
	if resp[1] != 0x5A { // 0x5A = Request Granted
		return fmt.Errorf("SOCKS4 connect ditolak dengan kode: 0x%02X", resp[1])
	}
	return nil
}

// ==========================================
// 3. SHARDED IN-MEMORY CACHE
// ==========================================

type cacheShard struct {
	sync.RWMutex
	items map[string]CacheItem
}

type ShardedCache struct {
	shards [shardCount]*cacheShard
	hits   uint64
	misses uint64
}

func NewShardedCache() *ShardedCache {
	c := &ShardedCache{}
	for i := 0; i < shardCount; i++ {
		c.shards[i] = &cacheShard{
			items: make(map[string]CacheItem),
		}
	}
	return c
}

func (c *ShardedCache) getShard(key string) *cacheShard {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return c.shards[uint(h.Sum32())%shardCount]
}

func (c *ShardedCache) Get(key string) (*CacheEntry, bool) {
	shard := c.getShard(key)
	shard.RLock()
	val, found := shard.items[key]
	shard.RUnlock()

	if !found {
		atomic.AddUint64(&c.misses, 1)
		return nil, false
	}

	atomic.AddUint64(&c.hits, 1)
	now := time.Now()
	return &CacheEntry{
		RawPayload: val.payload,
		ExpiresAt:  val.expiresAt,
		IsStale:    now.After(val.expiresAt),
	}, true
}

func (c *ShardedCache) Set(key string, payload []byte, ttl time.Duration) {
	shard := c.getShard(key)
	buf := make([]byte, len(payload))
	copy(buf, payload)

	shard.Lock()
	shard.items[key] = CacheItem{
		payload:   buf,
		expiresAt: time.Now().Add(ttl),
	}
	shard.Unlock()
}

func (c *ShardedCache) Prune() {
	now := time.Now()
	for _, shard := range c.shards {
		shard.Lock()
		for k, v := range shard.items {
			if now.Sub(v.expiresAt) > 2*time.Hour {
				delete(shard.items, k)
			}
		}
		shard.Unlock()
	}
}

// ==========================================
// 4. UPSTREAM RESOLVER WITH SOCKS TUNNEL
// ==========================================

type UpstreamClient struct {
	servers     []string
	socksClient *SOCKSClient
	timeout     time.Duration
}

func NewUpstreamClient(servers []string, socksClient *SOCKSClient, timeout time.Duration) *UpstreamClient {
	return &UpstreamClient{
		servers:     servers,
		socksClient: socksClient,
		timeout:     timeout,
	}
}

func (u *UpstreamClient) Exchange(ctx context.Context, query []byte, isTCP bool) ([]byte, error) {
	if len(u.servers) == 0 {
		return nil, errors.New("tidak ada upstream DNS server yang disetel")
	}

	var lastErr error
	for _, target := range u.servers {
		resp, err := u.send(ctx, target, query, isTCP)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("semua upstream gagal: %w", lastErr)
}

func (u *UpstreamClient) send(ctx context.Context, target string, query []byte, isTCP bool) ([]byte, error) {
	// Jika SOCKS Proxy aktif, alihkan DNS via TCP Tunnel melalui SOCKS
	if u.socksClient != nil && u.socksClient.ProxyAddr != "" {
		conn, err := u.socksClient.Dial(ctx, target)
		if err != nil {
			return nil, err
		}
		defer conn.Close()

		_ = conn.SetDeadline(time.Now().Add(u.timeout))

		var lenBuf [2]byte
		binary.BigEndian.PutUint16(lenBuf[:], uint16(len(query)))

		if _, err := conn.Write(append(lenBuf[:], query...)); err != nil {
			return nil, err
		}

		if _, err := io.ReadFull(conn, lenBuf[:]); err != nil {
			return nil, err
		}
		respLen := binary.BigEndian.Uint16(lenBuf[:])

		resp := make([]byte, respLen)
		if _, err := io.ReadFull(conn, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}

	// Direct Mode (Tanpa SOCKS)
	d := net.Dialer{Timeout: u.timeout}
	if isTCP {
		conn, err := d.DialContext(ctx, "tcp", target)
		if err != nil {
			return nil, err
		}
		defer conn.Close()

		var lenBuf [2]byte
		binary.BigEndian.PutUint16(lenBuf[:], uint16(len(query)))
		if _, err := conn.Write(append(lenBuf[:], query...)); err != nil {
			return nil, err
		}
		if _, err := io.ReadFull(conn, lenBuf[:]); err != nil {
			return nil, err
		}
		respLen := binary.BigEndian.Uint16(lenBuf[:])
		resp := make([]byte, respLen)
		if _, err := io.ReadFull(conn, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}

	// Direct UDP
	conn, err := d.DialContext(ctx, "udp", target)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if _, err := conn.Write(query); err != nil {
		return nil, err
	}

	resp := make([]byte, udpBufferSize)
	n, err := conn.Read(resp)
	if err != nil {
		return nil, err
	}
	return resp[:n], nil
}

// ==========================================
// 5. PROXY PIPELINE & REWRITING
// ==========================================

type Engine struct {
	cache    *ShardedCache
	upstream *UpstreamClient
	logger   *slog.Logger
}

func NewEngine(cache *ShardedCache, upstream *UpstreamClient, logger *slog.Logger) *Engine {
	return &Engine{
		cache:    cache,
		upstream: upstream,
		logger:   logger,
	}
}

func (e *Engine) Process(ctx context.Context, query []byte, isTCP bool) ([]byte, error) {
	var p dnsmessage.Parser
	header, err := p.Start(query)
	if err != nil {
		return nil, fmt.Errorf("packet corrupt: %w", err)
	}

	q, err := p.Question()
	if err != nil {
		return e.upstream.Exchange(ctx, query, isTCP)
	}

	cacheKey := fmt.Sprintf("%s:%d:%d", q.Name.String(), q.Type, q.Class)

	// Cek Cache Lokal
	if entry, found := e.cache.Get(cacheKey); found {
		res := make([]byte, len(entry.RawPayload))
		copy(res, entry.RawPayload)
		binary.BigEndian.PutUint16(res[0:2], header.ID) // Match TxID

		if entry.IsStale {
			go e.revalidateAsync(cacheKey, query, isTCP)
		}
		return res, nil
	}

	// Cache Miss -> Forwarding via SOCKS/Upstream
	resp, err := e.upstream.Exchange(ctx, query, isTCP)
	if err != nil {
		return nil, err
	}

	go e.storeInCache(cacheKey, resp)
	return resp, nil
}

func (e *Engine) revalidateAsync(key string, query []byte, isTCP bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := e.upstream.Exchange(ctx, query, isTCP)
	if err != nil {
		return
	}
	e.storeInCache(key, resp)
}

func (e *Engine) storeInCache(key string, resp []byte) {
	var p dnsmessage.Parser
	if _, err := p.Start(resp); err != nil {
		return
	}
	_ = p.SkipAllQuestions()

	minTTL := uint32(300)
	hasAnswers := false

	for {
		ah, err := p.AnswerHeader()
		if err != nil {
			break
		}
		hasAnswers = true
		if ah.TTL > 0 && (ah.TTL < minTTL || minTTL == 300) {
			minTTL = ah.TTL
		}
		if err := p.SkipAnswer(); err != nil {
			break
		}
	}

	if hasAnswers {
		if minTTL > 86400 {
			minTTL = 86400
		}
		e.cache.Set(key, resp, time.Duration(minTTL)*time.Second)
	}
}

// ==========================================
// 6. SERVER LISTENERS
// ==========================================

type Server struct {
	addr    string
	engine  *Engine
	logger  *slog.Logger
	udpPool sync.Pool
}

func NewServer(addr string, engine *Engine, logger *slog.Logger) *Server {
	return &Server{
		addr:   addr,
		engine: engine,
		logger: logger,
		udpPool: sync.Pool{
			New: func() any {
				b := make([]byte, udpBufferSize)
				return &b
			},
		},
	}
}

func (s *Server) StartUDP(ctx context.Context) error {
	pc, err := net.ListenPacket("udp", s.addr)
	if err != nil {
		return err
	}
	defer pc.Close()

	go func() {
		<-ctx.Done()
		_ = pc.Close()
	}()

	s.logger.Info("UDP Server Aktif", "addr", s.addr)

	for {
		bufPtr := s.udpPool.Get().(*[]byte)
		n, clientAddr, err := pc.ReadFrom(*bufPtr)
		if err != nil {
			s.udpPool.Put(bufPtr)
			select {
			case <-ctx.Done():
				return nil
			default:
				continue
			}
		}

		req := make([]byte, n)
		copy(req, (*bufPtr)[:n])
		s.udpPool.Put(bufPtr)

		go func(src net.Addr, queryPayload []byte) {
			resp, err := s.engine.Process(context.Background(), queryPayload, false)
			if err != nil {
				return
			}
			_, _ = pc.WriteTo(resp, src)
		}(clientAddr, req)
	}
}

func (s *Server) StartTCP(ctx context.Context) error {
	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	defer l.Close()

	go func() {
		<-ctx.Done()
		_ = l.Close()
	}()

	s.logger.Info("TCP Server Aktif", "addr", s.addr)

	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				continue
			}
		}

		go s.handleTCP(conn)
	}
}

func (s *Server) handleTCP(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	var lenBuf [2]byte
	for {
		if _, err := io.ReadFull(conn, lenBuf[:]); err != nil {
			return
		}
		qLen := binary.BigEndian.Uint16(lenBuf[:])

		req := make([]byte, qLen)
		if _, err := io.ReadFull(conn, req); err != nil {
			return
		}

		resp, err := s.engine.Process(context.Background(), req, true)
		if err != nil {
			return
		}

		binary.BigEndian.PutUint16(lenBuf[:], uint16(len(resp)))
		if _, err := conn.Write(append(lenBuf[:], resp...)); err != nil {
			return
		}
	}
}

// ==========================================
// 7. MAIN FUNCTION & CLI INTEGRATION
// ==========================================

func main() {
	listenFlag := flag.String("listen", defaultListenAddr, "IP:Port lokal listener DNS")
	upstreamFlag := flag.String("upstreams", defaultUpstreams, "Upstream DNS dipisah koma (contoh: 1.1.1.1:53,8.8.8.8:53)")
	socksFlag := flag.String("socks", "", "Alamat SOCKS Proxy (contoh: 127.0.0.1:1080 atau kosong jika direct)")
	socksVerFlag := flag.Int("socks-ver", 5, "Versi protokol SOCKS (4 atau 5)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var socksClient *SOCKSClient
	if *socksFlag != "" {
		socksClient = &SOCKSClient{
			ProxyAddr: *socksFlag,
			Version:   *socksVerFlag,
			Timeout:   3 * time.Second,
		}
		logger.Info("Mengaktifkan SOCKS Tunnel", "proxy", *socksFlag, "protocol_version", *socksVerFlag)
	} else {
		logger.Info("Mode Direct (Tanpa SOCKS Proxy)")
	}

	cache := NewShardedCache()
	upstreams := strings.Split(*upstreamFlag, ",")
	client := NewUpstreamClient(upstreams, socksClient, 3*time.Second)
	engine := NewEngine(cache, client, logger)
	server := NewServer(*listenFlag, engine, logger)

	// Statistik setiap 30 detik
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cache.Prune()
				hits := atomic.LoadUint64(&cache.hits)
				misses := atomic.LoadUint64(&cache.misses)
				var ratio float64
				if hits+misses > 0 {
					ratio = (float64(hits) / float64(hits+misses)) * 100
				}
				logger.Info("Statistik Cache", "Hits", hits, "Misses", misses, "Hit_Ratio", fmt.Sprintf("%.2f%%", ratio))
			case <-ctx.Done():
				return
			}
		}
	}()

	// Start UDP
	go func() {
		if err := server.StartUDP(ctx); err != nil {
			logger.Error("UDP fatal error", "err", err)
			cancel()
		}
	}()

	// Start TCP
	go func() {
		if err := server.StartTCP(ctx); err != nil {
			logger.Error("TCP fatal error", "err", err)
			cancel()
		}
	}()

	logger.Info("DNS Proxy Siap!", "listen", *listenFlag, "upstreams", upstreams)
	<-ctx.Done()

	logger.Info("Mematikan proxy secara aman...")
	time.Sleep(300 * time.Millisecond)
	logger.Info("Proxy berhasil dimatikan.")
}
