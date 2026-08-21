package gateway

import (
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"nganjuk-engine-reborn/config"
	"nganjuk-engine-reborn/logic"
)

func handleTCPConnection(conn net.Conn) {
	config.BytesMutex.Lock(); config.ActiveConns++; config.BytesMutex.Unlock()
	defer func() { config.BytesMutex.Lock(); config.ActiveConns--; config.BytesMutex.Unlock(); conn.Close() }()

	buf := make([]byte, 256)
	if _, err := io.ReadFull(conn, buf[:2]); err != nil { return }
	if buf[0] != 0x05 { return }
	
	nMethods := int(buf[1])
	if _, err := io.ReadFull(conn, buf[:nMethods]); err != nil { return }
	
	// 🛑 KEMBALI KE SOCKS5 MURNI TANPA PASSWORD! (Jalan Tol Terbuka)
	conn.Write([]byte{0x05, 0x00})

	if _, err := io.ReadFull(conn, buf[:4]); err != nil { return }
	cmd := buf[1]
	if cmd == 0x03 { HandleUDPAssociate(conn); return } 
	if cmd != 0x01 { conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0,0,0,0, 0,0}); return }

	var target string
	switch buf[3] {
	case 0x01:
		ip := make([]byte, 4); io.ReadFull(conn, ip); pBuf := make([]byte, 2); io.ReadFull(conn, pBuf)
		target = fmt.Sprintf("%d.%d.%d.%d:%d", ip[0], ip[1], ip[2], ip[3], int(pBuf[0])<<8|int(pBuf[1]))
	case 0x03:
		lBuf := make([]byte, 1); io.ReadFull(conn, lBuf); domain := make([]byte, int(lBuf[0])); io.ReadFull(conn, domain)
		pBuf := make([]byte, 2); io.ReadFull(conn, pBuf)
		target = fmt.Sprintf("%s:%d", string(domain), int(pBuf[0])<<8|int(pBuf[1]))
	default: return
	}

	targetLower := strings.ToLower(target)
	now := time.Now()

	config.StatsMutex.Lock()
	stats, exists := config.StatsMap[targetLower]
	if !exists { stats = config.ClientStats{Hits: 1, LastActive: now, BurstCount: 0}
	} else { stats.Hits++; timeDiff := now.Sub(stats.LastActive); if timeDiff < 150 * time.Millisecond { stats.BurstCount++ } else if timeDiff > 2 * time.Second { stats.BurstCount = 0 } }
	if strings.Contains(targetLower, "omi") || strings.Contains(targetLower, "litmatch") { config.LastAnchorDating = now }
	if strings.Contains(targetLower, "telegram") || strings.Contains(targetLower, "t.me") { config.LastAnchorTele = now }
	activeAnchor := config.LastAnchorDating
	if config.LastAnchorTele.After(config.LastAnchorDating) { activeAnchor = config.LastAnchorTele }
	lastActiveBackup := stats.LastActive
	stats.LastActive = now
	config.StatsMap[targetLower] = stats
	config.StatsMutex.Unlock()

	verdict := logic.EvaluateTarget(target, stats.Hits, stats.BurstCount, lastActiveBackup, activeAnchor)
	if verdict == "REGULAR" && IsCacheableAsset(targetLower) { config.LogMutex.Lock(); config.CacheHits++; config.LogMutex.Unlock(); verdict = "ALLOW_SYSTEM" }
	if verdict == "REGULAR" && IsStreamingVideo(targetLower) { verdict = "ALLOW_SYSTEM" }

	// 🌍 SUNTIK BENDERA NEGARA SEBELUM DI-LOG KE DASHBOARD!
	flag := logic.GetGeoFlag(target)
	displayTarget := flag + " " + target
	go config.AddLog(displayTarget, verdict)

	if verdict == "DROP" { conn.Write([]byte{0x05, 0x02, 0x00, 0x01, 0,0,0,0, 0,0}); return }

	destConn, err := net.DialTimeout("tcp", target, 5*time.Second)
	if err != nil { conn.Write([]byte{0x05, 0x04, 0x00, 0x01, 0,0,0,0, 0,0}); return }
	defer destConn.Close()

	if verdict == "ALLOW_VVIP" || verdict == "ALLOW_SYSTEM" {
		if tcpConn, ok := conn.(*net.TCPConn); ok { tcpConn.SetReadBuffer(512 * 1024); tcpConn.SetWriteBuffer(512 * 1024); tcpConn.SetNoDelay(true) }
		if tcpDestConn, ok := destConn.(*net.TCPConn); ok { tcpDestConn.SetReadBuffer(512 * 1024); tcpDestConn.SetWriteBuffer(512 * 1024); tcpDestConn.SetNoDelay(true) }
	}

	conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0,0,0,0, 0,0})
	SyncChunkedPipe(conn, destConn)
}
