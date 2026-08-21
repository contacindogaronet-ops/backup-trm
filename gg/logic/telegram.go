package logic

import (
	"net"
	"strings"
)

// IsTelegramTraffic mendeteksi apakah paket berasal dari ekosistem Telegram
func IsTelegramTraffic(target string) bool {
	targetLower := strings.ToLower(target)
	
	// 1. Cek Domain Standar Telegram
	teleDomains := []string{"telegram", "t.me", "stel.com", "tg.dev"}
	for _, d := range teleDomains {
		if strings.Contains(targetLower, d) { 
			return true 
		}
	}

	// 2. Cek Blok IP ASN Telegram (Raw IP MTProto Bypass)
	host, _, err := net.SplitHostPort(target)
	if err != nil { host = target } // Fallback jika tidak ada port
	
	ip := net.ParseIP(host)
	if ip != nil {
		// Daftar Sakti Subnet IP Data Center Telegram Global
		teleSubnets := []string{
			"91.108.4.0/22", 
			"91.108.8.0/22", 
			"91.108.12.0/22", 
			"91.108.16.0/22",
			"91.108.20.0/22", 
			"91.108.56.0/22", 
			"149.154.160.0/20", 
			"185.76.151.0/24",
		}
		
		for _, cidr := range teleSubnets {
			_, subnet, _ := net.ParseCIDR(cidr)
			if subnet != nil && subnet.Contains(ip) {
				return true
			}
		}
	}
	
	return false
}
