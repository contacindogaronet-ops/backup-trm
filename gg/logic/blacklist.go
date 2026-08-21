package logic

import "strings"

func IsKnownAdTracker(targetLower string) bool {
	hellSword := []string{
		// 💀 TRACKER LAMA
		"googleads", "doubleclick", "applovin", "unity3d", "vungle",
		"adcolony", "inmobi", "appsflyer", "branch.io", "adjust.com",
		"crashlytics", "facebook-net", "adservice", "syndication", "safeframe",
		
		// 🩸 NEW TARGETS (DARI HASIL RADAR NGANJUK LU!)
		"pangle.io",       // TikTok/ByteDance Ads Network
		"pglstatp.com",    // ByteDance Tracking/Static Ads
		"bttss.com",       // ByteDance Telemetry
		"sentry.io",       // Error & Telemetry Tracker
		"analytics.",      // Semua subdomain yang depannya analytics.
		"byteoversight",   // TikTok Hidden Tracker
		"goalempire.vip",  // Domain scam/ads dari screenshot lu
	}

	for _, ad := range hellSword {
		if strings.Contains(targetLower, ad) { 
			return true 
		}
	}
	return false
}
