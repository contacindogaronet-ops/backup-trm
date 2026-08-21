package main

import (
	"net"
	"sync"
	"time"
)

// Ukuran buffer optimal anti-bufferbloat & ramah L1/L2 Cache ARM64
var (
	RegulerPool = sync.Pool{
		New: func() any {
			b := make([]byte, 16*1024) // 16KB untuk browsing umum & latensi minimal
			return &b
		},
	}
	VVIPPool = sync.Pool{
		New: func() any {
			b := make([]byte, 64*1024) // 64KB: Sweet spot throughput maksimal tanpa lonjakan jitter
			return &b
		},
	}
)

// TuneSocket mengaktifkan mode latensi ultra-rendah dan menyerahkan buffer scaling ke Linux Kernel
func TuneSocket(c net.Conn) {
	if tcp, ok := c.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true) // Matikan Nagle Algorithm (Hilangkan delay paket)
		_ = tcp.SetKeepAlive(true)
		_ = tcp.SetKeepAlivePeriod(30 * time.Second)
		// CATATAN KRUSIAL: Jangan panggil SetReadBuffer/SetWriteBuffer secara statis!
		// Biarkan Linux TCP Auto-Tuning bekerja dinamis untuk mencegah Bufferbloat & Jitter.
	}
}

// RelayTCP melakukan transfer data zero-latency dua arah dengan immediate flush
func RelayTCP(client, target net.Conn, tier byte, telemetry *TelemetryTracker) {
	TuneSocket(client)
	TuneSocket(target)

	var pool *sync.Pool
	if tier == CodeVVIP {
		pool = &VVIPPool
	} else {
		pool = &RegulerPool
	}

	var wg sync.WaitGroup
	wg.Add(2)

	pipe := func(dst, src net.Conn) {
		defer wg.Done()
		bufPtr := pool.Get().(*[]byte)
		defer pool.Put(bufPtr)

		buf := *bufPtr
		for {
			nr, readErr := src.Read(buf)
			if nr > 0 {
				nw, writeErr := dst.Write(buf[:nr])
				if nw > 0 {
					telemetry.BytesTransferred.Add(uint64(nw))
				}
				if writeErr != nil {
					break
				}
			}
			if readErr != nil {
				break
			}
		}

		// Half-close graceful handshake teardown
		if tc, ok := dst.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		} else {
			_ = dst.Close()
		}
	}

	go pipe(target, client)
	go pipe(client, target)

	wg.Wait()
	_ = client.Close()
	_ = target.Close()
}
