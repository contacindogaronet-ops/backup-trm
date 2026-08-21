package main

import (
	"encoding/binary"
	"net"
	"strconv"
)

// ==========================================
// ZERO-ALLOC UDP RELAY SOCKS5
// ==========================================
func handleUDPRelay(conn *net.UDPConn) {
	var buf [4096]byte
	for {
		n, clientAddr, err := conn.ReadFromUDP(buf[:])
		if err != nil || n < 4 {
			continue
		}

		if buf[0] != 0 || buf[1] != 0 || buf[2] != 0 {
			continue
		}

		atyp := buf[3]
		idx := 4
		var destIP net.IP
		var destPort uint16

		switch atyp {
		case 0x01:
			if n < idx+6 {
				continue
			}
			destIP = net.IP(buf[idx : idx+4])
			destPort = binary.BigEndian.Uint16(buf[idx+4 : idx+6])
			idx += 6
		case 0x03:
			continue
		case 0x04:
			if n < idx+18 {
				continue
			}
			destIP = net.IP(buf[idx : idx+16])
			destPort = binary.BigEndian.Uint16(buf[idx+16 : idx+18])
			idx += 18
		default:
			continue
		}

		if destPort == 53 {
			TotalBlocked.Add(1)
			continue
		}

		// 🔴 FILTERING UDP SOCKS5 (Panggilan ulang checkAccess dari socks.go)
		if !checkAccess([]byte(destIP.String()), destIP, destIP.String()+":"+strconv.Itoa(int(destPort))) {
			continue // Diam-diam menjatuhkan paket jika ditolak
		}

		targetUDP := &net.UDPAddr{IP: destIP, Port: int(destPort)}
		payload := buf[idx:n]
		rawConn, err := net.DialUDP("udp", nil, targetUDP)
		if err == nil {
			rawConn.Write(payload)
			_ = GPool.Submit(func() {
				defer rawConn.Close()
				var backBuf [4096]byte
				backN, _ := rawConn.Read(backBuf[idx:])
				if backN > 0 {
					copy(backBuf[:idx], buf[:idx])
					conn.WriteToUDP(backBuf[:idx+backN], clientAddr)
				}
			})
		}
	}
}
