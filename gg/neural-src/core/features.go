package core

import (
	"net"
	"time"
)

type Features struct {
	IntervalFeature float64
	HitFeature      float64
	AnchorFeature   float64
	RawIPFeature    float64 // 👁️ MATA KE-4: RADAR SDK IKLAN!
}

func ExtractFeatures(target string, hits int, lastActive, lastAnchor time.Time) Features {
	now := time.Now()
	
	// 1. Fitur Interval
	intervalF := now.Sub(lastActive).Seconds() / 10.0
	if intervalF > 1.0 { intervalF = 1.0 }

	// 2. Fitur Hit Spam
	hitF := float64(hits) / 100.0
	if hitF > 1.0 { hitF = 1.0 }

	// 3. Fitur Anchor (Reward VVIP)
	anchorF := 0.0
	if now.Sub(lastAnchor).Seconds() < 10.0 { anchorF = 1.0 }

	// 4. MATA KE-4: Deteksi IP Telanjang (Bypass DNS dari Ad SDK)
	rawIPF := 0.0
	host, _, err := net.SplitHostPort(target)
	if err != nil { host = target } // Fallback

	// Jika target bisa di-parse sebagai IP (IPv4 / IPv6 murni), berarti bukan Domain!
	if net.ParseIP(host) != nil {
		rawIPF = 1.0 // Suspicion MAX! Kemungkinan besar SDK Iklan!
	}

	return Features{
		IntervalFeature: intervalF, 
		HitFeature:      hitF, 
		AnchorFeature:   anchorF, 
		RawIPFeature:    rawIPF,
	}
}
