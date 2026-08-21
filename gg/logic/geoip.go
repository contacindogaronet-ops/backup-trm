package logic

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

var geoCache = make(map[string]string)
var geoMutex sync.Mutex
var httpClientGeo = &http.Client{Timeout: 2 * time.Second}

func GetGeoFlag(target string) string {
	parts := strings.Split(target, ":")
	ip := parts[0]

	// Bypass IP Lokal biar gak nyepam API
	if strings.HasPrefix(ip, "127.") || strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") { return "🏠" }

	geoMutex.Lock()
	flag, exists := geoCache[ip]
	geoMutex.Unlock()
	if exists { return flag }

	// Tandai sedang di-fetch biar gak double request
	geoMutex.Lock()
	geoCache[ip] = "🌍"
	geoMutex.Unlock()

	// ⚡ Sedot bendera negara tanpa memblokir koneksi internet lu!
	go func(ipToFetch string) {
		resp, err := httpClientGeo.Get("http://ip-api.com/json/" + ipToFetch + "?fields=countryCode")
		if err == nil {
			defer resp.Body.Close()
			var res struct { CountryCode string `json:"countryCode"` }
			if json.NewDecoder(resp.Body).Decode(&res) == nil && res.CountryCode != "" && len(res.CountryCode) == 2 {
				cc := strings.ToUpper(res.CountryCode)
				
				// 🛠️ FIX OVERFLOW: Ubah cc jadi rune DULU, baru ditambah angka sakti!
				emoji := string(rune(cc[0]) + 127397) + string(rune(cc[1]) + 127397)
				
				geoMutex.Lock(); geoCache[ipToFetch] = emoji; geoMutex.Unlock()
			} else {
				geoMutex.Lock(); geoCache[ipToFetch] = "🌐"; geoMutex.Unlock()
			}
		}
	}(ip)

	return "🌍"
}
