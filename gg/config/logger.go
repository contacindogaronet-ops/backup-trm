package config

import (
	"strings"
	"time"
)

func AddLog(target string, verdict string) {
	LogMutex.Lock()
	defer LogMutex.Unlock()

	// Update Counter Spesifik
	if verdict == "DROP" { AdsBlocked++ }
	if verdict == "ALLOW_VVIP" || verdict == "ALLOW_SYSTEM" { VIPAllowed++ }
	
	tLower := strings.ToLower(target)
	if strings.Contains(tLower, "telegram") || strings.Contains(tLower, "t.me") { TeleHits++ }
	if strings.Contains(tLower, "omi") || strings.Contains(tLower, "litmatch") || strings.Contains(tLower, "tinder") { DatingHits++ }

	if verdict == "DROP" || verdict == "ALLOW_VVIP" { LastLearning = time.Now() }

	RealTrafficLogs = append([]TrafficLog{{
		Time:    time.Now().Format("15:04:05"),
		Target:  target,
		Verdict: verdict,
	}}, RealTrafficLogs...)

	if len(RealTrafficLogs) > 50 { RealTrafficLogs = RealTrafficLogs[:50] }
}

func IsAILearningActive() bool {
	LogMutex.Lock()
	defer LogMutex.Unlock()
	return time.Since(LastLearning) < 3*time.Second
}
