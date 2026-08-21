package main

import (
	"encoding/binary"
	"net"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

func readExactBytes(conn net.Conn, buf []byte) error {
	var totalRead int
	target := len(buf)
	for totalRead < target {
		n, err := conn.Read(buf[totalRead:])
		if n > 0 {
			totalRead += n
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func processTCP(client *net.TCPConn) {
	client.SetDeadline(time.Now().Add(10 * time.Second))
	var buf [262]byte // 🔴 SANGAT BAGUS: Stack allocation
	if err := readExactBytes(client, buf[:1]); err != nil {
		client.Close()
		return
	}
	version := buf[0]
	if version == 0x00 {
		client.Close()
		return
	}
	if version == 0x04 {
		handleSOCKS4(client, &buf)
	} else if version == 0x05 {
		handleSOCKS5(client, &buf)
	} else {
		client.Close()
	}
}

func getIPTier(ip net.IP) (byte, bool) {
	isV4 := ip.To4() != nil
	root := IPv6Trie
	searchIP := ip.To16()
	if isV4 {
		root = IPv4Trie
		searchIP = ip.To4()
	}
	node := root
	lastTier := byte(CodeReguler)
	found := false
	for i := 0; i < len(searchIP)*8; i++ {
		byteIndex := i / 8
		bitIndex := 7 - (i % 8)
		bit := (searchIP[byteIndex] >> bitIndex) & 1
		node = node.Children[bit]
		if node == nil {
			break
		}
		if node.HasRule {
			lastTier = node.Tier
			found = true
		}
	}
	return lastTier, found
}

// 🔴 FIX: ZERO-ALLOCATION FASTCACHE GET
func checkAccess(targetKey []byte, targetIP net.IP, targetStr string) (bool, byte) {
	var tier byte = CodeReguler
	var valBuf [1]byte // Stack buffer super kecil untuk nampung balasan Fastcache

	if targetIP != nil {
		if t, found := getIPTier(targetIP); found {
			tier = t
		}
	} else {
		// Gunakan valBuf[:0] agar Fastcache TIDAK me-malloc memori baru di Heap
		if val := DBEngine.Get(valBuf[:0], targetKey); len(val) > 0 {
			tier = val[0]
		} else {
			for i := 0; i < len(targetKey); i++ {
				if targetKey[i] == '.' && i+1 < len(targetKey) {
					if val := DBEngine.Get(valBuf[:0], targetKey[i+1:]); len(val) > 0 {
						tier = val[0]
						break
					}
				}
			}
		}
	}

	if StrictMode {
		if tier != CodeVVIP {
			TotalBlocked.Add(1)
			if LogUniversal || LogBlock {
				log.Warn().Str("target", targetStr).Msg("🛑 [STRICT] Akses Ditolak")
			}
			return false, tier
		}
		if LogUniversal || LogAllow {
			log.Info().Str("target", targetStr).Msg("👑 [KASTA VVIP] Lolos Strict Mode")
		}
		return true, tier
	}

	if tier == CodeBlock {
		TotalBlocked.Add(1)
		if LogUniversal || LogBlock {
			log.Warn().Str("target", targetStr).Msg("🛑 [KASTA SAMPAH] Iklan Dibantai")
		}
		return false, tier
	}
	
	if tier == CodeReguler {
		if LogUniversal || LogReguler {
			log.Info().Str("target", targetStr).Msg("🟢 [KASTA REGULER] Jalur Standar")
		}
	} else if tier == CodeVIP || tier == CodeVVIP {
		if LogUniversal || LogAllow {
			log.Info().Str("target", targetStr).Msg("👑 [KASTA VVIP] Jalur Prioritas Aktif")
		}
	}
	
	return true, tier
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
		if LogUniversal || LogBlock || LogErrors {
			log.Warn().Str("ip", ip.String()).Msg("🛑 SOCKS4 Blokir Port 53")
		}
		client.Write([]byte{0x00, 0x5b, 0x00, 0x00, 0, 0, 0, 0})
		client.Close()
		return
	}
	
	// Skip identd string (UserID)
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

	allowed, tier := checkAccess([]byte(ip.String()), ip, target)
	if !allowed {
		client.Write([]byte{0x00, 0x5b, 0x00, 0x00, 0, 0, 0, 0})
		client.Close()
		return
	}

	client.SetDeadline(time.Time{})
	reply := []byte{0x00, 0x5a, 0x00, 0x00, 0, 0, 0, 0}
	executeRelay(client, target, reply, tier)
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
		if LogUniversal || LogBlock || LogErrors {
			log.Warn().Str("target", target).Msg("🛑 SOCKS5 Blokir Port 53")
		}
		client.Write([]byte{0x05, 0x02, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		client.Close()
		return
	}

	if cmd == 0x03 {
		// 🔴 FIX: ZERO-ALLOCATION UDP REPLY
		// Gunakan ulang array 'buf' yang sudah ada, jangan bikin []byte baru.
		buf[0], buf[1], buf[2], buf[3] = 0x05, 0x00, 0x00, 0x01
		
		ip4 := ProxyIP.To4()
		if ip4 != nil {
			copy(buf[4:8], ip4)
			binary.BigEndian.PutUint16(buf[8:10], 2007)
			client.Write(buf[:10])
		} else {
			// Fallback jika bukan IPv4 (meski jarang)
			client.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		}
		
		client.SetDeadline(time.Time{})
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

	allowed, tier := checkAccess(targetKey, targetIP, target)
	if !allowed {
		client.Write([]byte{0x05, 0x02, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		client.Close()
		return
	}

	client.SetDeadline(time.Time{})
	reply := []byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
	executeRelay(client, target, reply, tier)
}
