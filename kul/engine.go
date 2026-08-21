package main

import (
	"context"
	"net"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
)

// Konstanta Kernel Linux TCP Fast Open (Server Side)
const TCP_FASTOPEN_SERVER = 23

// ==========================================
// 🔴 ARSITEKTUR LISTENER MODERN (SO_REUSEPORT + TFO)
// ==========================================
func StartEngine(addr string) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				// 1. Multi-Core Load Balancing (Anti-Bottleneck)
				_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEPORT, 1)
				
				// 2. TCP Fast Open Server (Queue 256)
				_ = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, TCP_FASTOPEN_SERVER, 256)
			})
		},
	}

	listener, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		log.Fatal().Err(err).Str("addr", addr).Msg("🔥 FATAL: Gagal buka port")
	}
	defer listener.Close()

	if EnableLog || DebugMode {
		log.Info().Str("addr", addr).Msg("🚀 Engine SOCKS5 Listener Aktif (TFO & ReusePort)")
	}

	// 🔴 THE STORM SHIELD: Mekanisme Backoff untuk melindungi CPU
	var tempDelay time.Duration

	// Loop penerima TCP murni (Epoll Netpoller)
	for {
		client, err := listener.Accept()
		if err != nil {
			// Jika OS kehabisan nafas (File Descriptor habis), kita beri jeda (Sleep) 
			// agar CPU Termux lu tidak Spike 100% karena Infinite Loop.
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				time.Sleep(tempDelay)
				continue
			}
			
			if DebugMode {
				log.Debug().Err(err).Msg("Engine: Accept failed")
			}
			continue
		}
		
		tempDelay = 0 // Reset perisai kalau OS udah normal

		tcpClient, ok := client.(*net.TCPConn)
		if ok {
			// 🔴 EARLY TUNING: Tembak NoDelay sejak dari pintu masuk!
			// Ini bikin proses parsing SOCKS5 di socks.go jalan tanpa antrean jaringan.
			tcpClient.SetNoDelay(true)
			
			// 🔴 GOROUTINE POOLING: Penahan Ledakan Memori
			// Jangan pakai 'go processTCP' sembarangan. Kita masukan ke dalam Pool kasta tinggi.
			task := func() {
				processTCP(tcpClient)
			}
			
			if err := GPool.Submit(task); err != nil {
				// Fallback darurat jika Pool benar-benar penuh
				go task()
			}
		}
	}
}
