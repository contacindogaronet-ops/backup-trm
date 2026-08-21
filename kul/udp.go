package main

import (
	"encoding/binary"
	"net"
	"strconv"
	"sync"
	"time"
)

var UDPSessions sync.Map

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

		// 🔴 TERIMA 2 RETURN VALUE DARI CHECKACCESS
		allowed, _ := checkAccess([]byte(destIP.String()), destIP, destIP.String()+":"+strconv.Itoa(int(destPort)))
		if !allowed {
			continue
		}

		targetUDP := &net.UDPAddr{IP: destIP, Port: int(destPort)}
		payload := buf[idx:n]
		
		sessionKey := clientAddr.String() + "|" + targetUDP.String()

		if val, ok := UDPSessions.Load(sessionKey); ok {
			val.(*net.UDPConn).Write(payload)
			continue
		}

		rawConn, err := net.DialUDP("udp", nil, targetUDP)
		if err != nil {
			continue
		}

		UDPSessions.Store(sessionKey, rawConn)
		rawConn.Write(payload)

		ActiveConns.Add(1)
		TotalAllowed.Add(1)

		_ = GPool.Submit(func() {
			defer rawConn.Close()
			defer UDPSessions.Delete(sessionKey)
			defer ActiveConns.Add(-1)

			var backBuf [4096]byte
			header := make([]byte, idx)
			copy(header, buf[:idx])

			for {
				rawConn.SetReadDeadline(time.Now().Add(2 * time.Minute))
				backN, err := rawConn.Read(backBuf[idx:])
				if err != nil || backN == 0 {
					break
				}
				
				copy(backBuf[:idx], header)
				conn.WriteToUDP(backBuf[:idx+backN], clientAddr)
			}
		})
	}
}
