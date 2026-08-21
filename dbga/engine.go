package main

import (
	"net"

	"github.com/rs/zerolog/log"
	"golang.org/x/sys/unix"
)

// ==========================================
// ENGINE LISTENER (TCP & UDP SOCKS)
// ==========================================
func StartEngine(addr string) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		log.Fatal().Err(err).Msg("TCP Addr invalid")
	}
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Fatal().Err(err).Msg("UDP Addr invalid")
	}

	tcpListener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		log.Fatal().Err(err).Msg("Gagal start TCP")
	}
	defer tcpListener.Close()

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal().Err(err).Msg("Gagal start UDP")
	}
	defer udpConn.Close()

	log.Info().Msgf("🔥 HYBRID SOCKS4/5 (TCP/UDP) ACTIVE AT %s", addr)

	_ = GPool.Submit(func() { handleUDPRelay(udpConn) })

	for {
		client, err := tcpListener.AcceptTCP()
		if err != nil {
			continue
		}
		tuneSocket(client)
		_ = GPool.Submit(func() { processTCP(client) })
	}
}

// ==========================================
// SOCKET TUNING
// ==========================================
func tuneSocket(conn *net.TCPConn) {
	sys, err := conn.SyscallConn()
	if err == nil {
		sys.Control(func(fd uintptr) {
			unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_RCVBUF, 16*1024*1024)
			unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_SNDBUF, 16*1024*1024)
			unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_NODELAY, 1)
			unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, 12, 1)
		})
	}
}

func readExactBytes(c *net.TCPConn, b []byte) error {
	total := 0
	for total < len(b) {
		n, err := c.Read(b[total:])
		if err != nil {
			return err
		}
		total += n
	}
	return nil
}
