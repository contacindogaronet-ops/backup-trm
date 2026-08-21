package main

import (
	"encoding/binary"
	"net"
	"strconv"

	"github.com/rs/zerolog/log"
)

// ==========================================
// TCP SOCKS4 & SOCKS5 PARSER (ZERO-ALLOC)
// ==========================================
func processTCP(client *net.TCPConn) {
	var buf [262]byte
	if err := readExactBytes(client, buf[:1]); err != nil {
		client.Close()
		return
	}

	version := buf[0]
	if version == 0x04 {
		handleSOCKS4(client, &buf)
	} else if version == 0x05 {
		handleSOCKS5(client, &buf)
	} else {
		client.Close()
	}
}

// 🔴 FUNGSI PENYARING KEBIJAKAN AKSES
func checkAccess(targetKey []byte, targetIP net.IP, targetStr string) bool {
	var tier byte = CodeReguler
	if val := DBEngine.Get(nil, targetKey); val != nil {
		tier = val[0]
	} else if targetIP != nil {
		for i := 0; i < len(IPRules); i++ {
			if IPRules[i].Net.Contains(targetIP) {
				tier = IPRules[i].Tier
				break
			}
		}
	}

	if StrictMode {
		if tier != CodeVVIP {
			TotalBlocked.Add(1)
			log.Warn().Str("target", targetStr).Msg("Strict Mode Blokir: Bukan Telegram")
			return false
		}
		return true
	}

	if tier == CodeBlock {
		TotalBlocked.Add(1)
		log.Warn().Str("target", targetStr).Msg("Blokir Reguler (Blacklist)")
		return false
	}
	return true
}

func handleSOCKS4(client *net.TCPConn, buf *[262]byte) {
	if err := readExactBytes(client, buf[1:8]); err != nil {
		client.Close()
		return
	}
	cmd := buf[1]
	port := binary.BigEndian.Uint16(buf[2:4])
	ip := net.IP(buf[4:8])

	if port == 53 {
		TotalBlocked.Add(1)
		log.Warn().Str("ip", ip.String()).Msg("SOCKS4 Blokir Port 53")
		client.Write([]byte{0x00, 0x5b, 0x00, 0x00, 0, 0, 0, 0})
		client.Close()
		return
	}

	var b [1]byte
	for {
		if err := readExactBytes(client, b[:]); err != nil || b[0] == 0x00 {
			break
		}
	}

	if cmd != 0x01 {
		client.Write([]byte{0x00, 0x5b, 0x00, 0x00, 0, 0, 0, 0})
		client.Close()
		return
	}

	target := ip.String() + ":" + strconv.Itoa(int(port))

	// 🔴 INTERCEPT SOCKS4 SEBELUM RELAY
	if !checkAccess([]byte(ip.String()), ip, target) {
		client.Write([]byte{0x00, 0x5b, 0x00, 0x00, 0, 0, 0, 0}) // Reject SOCKS4
		client.Close()
		return
	}

	executeRelay(client, target, []byte(ip.String()), ip)
}

func handleSOCKS5(client *net.TCPConn, buf *[262]byte) {
	if err := readExactBytes(client, buf[1:2]); err != nil {
		client.Close()
		return
	}
	numMethods := int(buf[1])
	if err := readExactBytes(client, buf[:numMethods]); err != nil {
		client.Close()
		return
	}
	client.Write([]byte{0x05, 0x00})

	if err := readExactBytes(client, buf[:4]); err != nil || buf[0] != 0x05 {
		client.Close()
		return
	}

	cmd := buf[1]
	atyp := buf[3]
	var target string
	var targetKey []byte
	var port uint16
	var targetIP net.IP

	switch atyp {
	case 0x01:
		if err := readExactBytes(client, buf[4:10]); err != nil {
			client.Close()
			return
		}
		port = binary.BigEndian.Uint16(buf[8:10])
		targetIP = net.IP(buf[4:8])
		target = targetIP.String() + ":" + strconv.Itoa(int(port))
		targetKey = []byte(targetIP.String())
	case 0x03:
		if err := readExactBytes(client, buf[4:5]); err != nil {
			client.Close()
			return
		}
		dlen := int(buf[4])
		if err := readExactBytes(client, buf[5:5+dlen+2]); err != nil {
			client.Close()
			return
		}
		port = binary.BigEndian.Uint16(buf[5+dlen : 5+dlen+2])
		domain := string(buf[5 : 5+dlen])
		target = domain + ":" + strconv.Itoa(int(port))
		targetKey = buf[5 : 5+dlen]
		targetIP = nil
	case 0x04:
		if err := readExactBytes(client, buf[4:22]); err != nil {
			client.Close()
			return
		}
		port = binary.BigEndian.Uint16(buf[20:22])
		targetIP = net.IP(buf[4:20])
		target = "[" + targetIP.String() + "]:" + strconv.Itoa(int(port))
		targetKey = []byte(targetIP.String())
	default:
		client.Close()
		return
	}

	if port == 53 {
		TotalBlocked.Add(1)
		log.Warn().Str("target", target).Msg("SOCKS5 Blokir Port 53")
		client.Write([]byte{0x05, 0x02, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		client.Close()
		return
	}

	if cmd == 0x03 {
		reply := []byte{0x05, 0x00, 0x00, 0x01}
		reply = append(reply, ProxyIP...)
		reply = binary.BigEndian.AppendUint16(reply, 2007)
		client.Write(reply)
		for {
			if err := readExactBytes(client, buf[:1]); err != nil {
				break
			}
		}
		client.Close()
		return
	}

	if cmd != 0x01 {
		client.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		client.Close()
		return
	}

	// 🔴 INTERCEPT SOCKS5 SEBELUM RELAY
	if !checkAccess(targetKey, targetIP, target) {
		client.Write([]byte{0x05, 0x02, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) // Reject SOCKS5
		client.Close()
		return
	}

	executeRelay(client, target, targetKey, targetIP)
}
