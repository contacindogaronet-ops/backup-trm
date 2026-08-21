package logic

import "strings"

func IsProtectedByShield(targetLower string) bool {
	// 🛑 Pengecualian: Jangan lindungi kalau ada bau-bau analytics/ads walaupun itu dari aplikasi VVIP
	if strings.Contains(targetLower, "analytics") || strings.Contains(targetLower, "log") {
		return false
	}

	holyShield := []string{
		"google", "gstatic", "googleapis", "youtube", "whatsapp", "wa.me",
		"fbcdn", "facebook", "instagram", "apple", "icloud", "cloudflare",
		"netflix", "spotify", "tiktok", "myrepublic",
		"ooklaserver", "speedtest", "fast.com", "nperf",
	}
	
	for _, domain := range holyShield {
		if strings.Contains(targetLower, domain) { return true }
	}
	return false
}
