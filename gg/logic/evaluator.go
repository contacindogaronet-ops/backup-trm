package logic

import (
	"strings"
	"time"

	"nganjuk-engine-reborn/config"
	"nganjuk-engine-reborn/dns"
)

// 💾 AddDynamicRule sekarang nyimpen datanya langsung ke SERVER STATE (config.go)
// Biar aturan lu abadi dan gak ilang waktu browser di-refresh!
func AddDynamicRule(target, verdict string) {
	config.RulesMutex.Lock()
	config.DynamicRules[target] = verdict
	config.RulesMutex.Unlock()
}

func EvaluateTarget(target string, hits int, burstCount int, lastActive, lastAnchor time.Time) string {
	targetLower := strings.ToLower(target)
	
	// 👑 LAYER -1: TITAH MUTLAK DARI SEPUH (SERVER-SIDE OVERRIDE)
	config.RulesMutex.Lock()
	
	// Cek kecocokan persis dulu
	ruleVerdict, exists := config.DynamicRules[targetLower]
	
	// Kalau gak ada yang persis, cek substring (misal blokir "ads", maka "api.ads.com" ikut kena)
	if !exists {
		for ruleTarget, v := range config.DynamicRules {
			if strings.Contains(targetLower, ruleTarget) {
				ruleVerdict = v
				exists = true
				break
			}
		}
	}
	config.RulesMutex.Unlock()

	// Kalau domain ini udah masuk daftar hitam/putih lu di web, langsung eksekusi!
	if exists {
		return ruleVerdict
	}

	// 🛠️ LAYER 0.1: BYPASS DNS TRAFFIC (Biar resolving gak nyangkut)
	if dns.IsDNSTraffic(targetLower) { 
		return "ALLOW_SYSTEM" 
	}
	
	// ✈️ LAYER 0.5: TELEGRAM FAST-PATH
	if IsTelegramTraffic(targetLower) { 
		return "ALLOW_VVIP" 
	}
	
	// 🛡️ LAYER 1: HOLY SHIELD (Whitelist Sistem & Apple/Google)
	if IsProtectedByShield(targetLower) { 
		return "ALLOW_SYSTEM" 
	}
	
	// 💀 LAYER 2: HELL SWORD (Algojo Ad Trackers Bawaan)
	if IsKnownAdTracker(targetLower) { 
		return "DROP" 
	}
	
	// ⚡ LAYER 2.5: BURST RADAR (Bypass Downloader / Speedtest)
	// Kalau nembak lebih dari 3 socket barengan di bawah 150ms, auto los!
	if burstCount >= 3 { 
		return "ALLOW_SYSTEM" 
	}
	
	// 🧠 LAYER 3: NEURAL KOGNITIF (AI yang mikir buat sisanya)
	return AskAI(target, hits, lastActive, lastAnchor)
}
