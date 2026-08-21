package main

import (
	"io"
	"net"
	"sync"
	"time"
)

var (
	RegulerPool = sync.Pool{
		New: func() any {
			b := make([]byte, 32*1024) // 32KB
			return &b
		},
	}
	VVIPPool = sync.Pool{
		New: func() any {
			b := make([]byte, 64*1024) // 64KB (Optimal ARM64)
			return &b
		},
	}
)

func TuneSocket(c net.Conn) {
	if tcp, ok := c.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
		_ = tcp.SetKeepAlive(true)
		_ = tcp.SetKeepAlivePeriod(30 * time.Second)
	}
}

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

		n, _ := io.CopyBuffer(dst, src, *bufPtr)
		if n > 0 {
			telemetry.BytesTransferred.Add(uint64(n))
		}

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
