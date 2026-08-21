package main

import (
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type UDPRelayHandler struct {
	zlog      zerolog.Logger
	telemetry *TelemetryTracker
}

func NewUDPRelayHandler(logger zerolog.Logger, t *TelemetryTracker) *UDPRelayHandler {
	return &UDPRelayHandler{
		zlog:      logger,
		telemetry: t,
	}
}

func (u *UDPRelayHandler) HandleUDPAssociate(clientConn net.Conn, req *SOCKS5Request) {
	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.3:0")
	if err != nil {
		u.zlog.Error().Err(err).Msg("failed resolving udp addr")
		_ = clientConn.Close()
		return
	}

	udpListener, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		u.zlog.Error().Err(err).Msg("failed binding udp listener")
		_ = clientConn.Close()
		return
	}
	defer udpListener.Close()

	assignedPort := uint16(udpListener.LocalAddr().(*net.UDPAddr).Port)
	if err := SendSocksUDPResponse(clientConn, assignedPort); err != nil {
		_ = clientConn.Close()
		return
	}

	u.telemetry.ActiveConns.Add(1)
	defer u.telemetry.ActiveConns.Add(-1)

	var closeOnce sync.Once
	done := make(chan struct{})

	go func() {
		buf := make([]byte, 1)
		for {
			if _, err := clientConn.Read(buf); err != nil {
				closeOnce.Do(func() { close(done) })
				return
			}
		}
	}()

	buf := make([]byte, 65535)
	for {
		select {
		case <-done:
			return
		default:
			_ = udpListener.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, srcAddr, err := udpListener.ReadFromUDP(buf)
			if err != nil {
				continue
			}

			if n < 3 {
				continue
			}

			if buf[2] != 0x00 {
				continue
			}

			atyp := buf[3]
			dataOffset := 4
			var targetHost string

			switch atyp {
			case AtypIPv4:
				if n < dataOffset+4+2 {
					continue
				}
				targetHost = net.IP(buf[dataOffset : dataOffset+4]).String()
				dataOffset += 4
			case AtypDomain:
				dLen := int(buf[dataOffset])
				dataOffset++
				if n < dataOffset+dLen+2 {
					continue
				}
				targetHost = string(buf[dataOffset : dataOffset+dLen])
				dataOffset += dLen
			case AtypIPv6:
				if n < dataOffset+16+2 {
					continue
				}
				targetHost = net.IP(buf[dataOffset : dataOffset+16]).String()
				dataOffset += 16
			default:
				continue
			}

			targetPort := uint16(buf[dataOffset])<<8 | uint16(buf[dataOffset+1])
			dataOffset += 2

			payload := buf[dataOffset:n]
			go u.forwardUDPPacket(udpListener, srcAddr, targetHost, targetPort, payload)
		}
	}
}

func (u *UDPRelayHandler) forwardUDPPacket(listener *net.UDPConn, clientAddr *net.UDPAddr, host string, port uint16, data []byte) {
	dstAddrStr := FormatAddrPort(host, port)
	rAddr, err := net.ResolveUDPAddr("udp", dstAddrStr)
	if err != nil {
		return
	}

	outConn, err := net.DialUDP("udp", nil, rAddr)
	if err != nil {
		return
	}
	defer outConn.Close()

	if _, err := outConn.Write(data); err != nil {
		return
	}

	respBuf := make([]byte, 65535)
	_ = outConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	rn, err := outConn.Read(respBuf)
	if err != nil {
		return
	}

	header := []byte{0x00, 0x00, 0x00, AtypIPv4, 127, 0, 0, 3, byte(port >> 8), byte(port & 0xFF)}
	finalPacket := append(header, respBuf[:rn]...)
	_, _ = listener.WriteToUDP(finalPacket, clientAddr)
}