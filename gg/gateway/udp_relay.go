package gateway

import (
	"io"
	"net"
	"time"
)

func HandleUDPAssociate(conn net.Conn) {
	bindAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	relay, err := net.ListenUDP("udp", bindAddr)
	if err != nil { conn.Write([]byte{0x05, 0x01, 0x00, 0x01, 0,0,0,0, 0,0}); return }
	defer relay.Close()

	port := relay.LocalAddr().(*net.UDPAddr).Port
	// Kasih tahu client lewat jalur mana data UDP harus ditembakkan
	conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 127, 0, 0, 1, byte(port >> 8), byte(port & 0xff)})

	go func() {
		buf := make([]byte, 65535)
		for {
			n, clientAddr, err := relay.ReadFromUDP(buf)
			if err != nil { break }
			if n < 10 { continue } // Header minimal SOCKS5 UDP adalah 10 bytes

			atyp := buf[3]
			var dstIP net.IP
			var dstPort int
			headerLen := 0

			// 🔓 BONGKAR HEADER ENKAPSULASI SOCKS5 UDP ASLI
			if atyp == 0x01 { // IPv4 Target
				dstIP = buf[4:8]
				dstPort = int(buf[8])<<8 | int(buf[9])
				headerLen = 10
			} else if atyp == 0x03 { // Domain Name Target
				dLen := int(buf[4])
				if n < 5+dLen+2 { continue }
				domain := string(buf[5 : 5+dLen])
				resolved, _ := net.ResolveIPAddr("ip", domain)
				if resolved != nil { dstIP = resolved.IP }
				dstPort = int(buf[5+dLen])<<8 | int(buf[6+dLen])
				headerLen = 5 + dLen + 2
			}

			if dstIP == nil { continue }

			payload := buf[headerLen:n]
			headerBackup := make([]byte, headerLen)
			copy(headerBackup, buf[:headerLen])

			// Tembakkan data asli tanpa header SOCKS ke internet lepas
			go func(cAddr *net.UDPAddr, tAddr *net.UDPAddr, pld, hdr []byte) {
				tConn, err := net.DialUDP("udp", nil, tAddr)
				if err != nil { return }
				defer tConn.Close()

				tConn.SetDeadline(time.Now().Add(4 * time.Second))
				tConn.Write(pld)

				respBuf := make([]byte, 65535)
				rn, _, err := tConn.ReadFromUDP(respBuf)
				if err == nil {
					// 🔒 BUNGKUS ULANG PAKET DENGAN SOCKS5 HEADER SEBELUM DIKEMBALIKAN KE CLIENT
					replyPacket := append(hdr, respBuf[:rn]...)
					relay.WriteToUDP(replyPacket, cAddr)
				}
			}(clientAddr, &net.UDPAddr{IP: dstIP, Port: dstPort}, payload, headerBackup)
		}
	}()

	// Tahan koneksi kendali TCP agar pipa UDP Associate tidak hangus di-close client
	io.Copy(io.Discard, conn)
}
