package gateway

import (
	"strings"
	"sync"
	"time"
)

type CacheItem struct {
	Body       []byte
	ContentType string
	Timestamp  time.Time
}

var (
	InMemoryCache  = make(map[string]CacheItem)
	CacheMutex     sync.Mutex
	MaxCacheSize   = 2 * 1024 * 1024 
)

// 📊 JALUR 1: Khusus Aset Ringan Website (Biar hemat bandwidth)
func IsCacheableAsset(url string) bool {
	urlLower := strings.ToLower(url)

	// Jangan cache file video ke memori RAM! (Biar HP ga meledak)
	if IsStreamingVideo(urlLower) {
		return false
	}

	lightweightExtensions := []string{
		".jpg", ".jpeg", ".png", ".gif", ".webp", ".ico",
		".css", ".js", ".json", ".woff2", ".woff", ".ttf",
	}
	for _, ext := range lightweightExtensions {
		if strings.Contains(urlLower, ext) {
			return true
		}
	}
	return false
}

// 🎬 JALUR 2: RADAR DETEKSI STREAMING VIDEO (IDE ULTRA ENTERPRISE LU!)
// Fungsi ini mendeteksi segala format video streaming di internet & Telegram CDN
func IsStreamingVideo(urlLower string) bool {
	videoPatterns := []string{
		".mp4", ".m3u8", ".ts", ".mkv", ".avi", ".flv", ".webm", ".mpeg",
		"/video/", "stream", "chunk", "cdn", "telecdn", "media",
	}
	for _, pattern := range videoPatterns {
		if strings.Contains(urlLower, pattern) {
			return true
		}
	}
	return false
}
