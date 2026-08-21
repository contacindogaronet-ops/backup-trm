package dns

import "strings"

func IsDNSTraffic(targetLower string) bool {
	if strings.HasSuffix(targetLower, ":53") || strings.HasSuffix(targetLower, ":853") {
		return true
	}
	dnsIPs := []string{"1.1.1.1", "1.0.0.1", "8.8.8.8", "8.8.4.4", "9.9.9.9"}
	for _, ip := range dnsIPs {
		if strings.Contains(targetLower, ip) { return true }
	}
	return false
}
