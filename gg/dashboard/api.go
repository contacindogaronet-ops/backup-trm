package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"time"

	"nganjuk-engine-reborn/config"
	"nganjuk-engine-reborn/logic"
)

func handleStats(w http.ResponseWriter, r *http.Request) {
	config.BytesMutex.Lock()
	// Sort Top Talkers
	type kv struct {
		Key   string
		Value int64
	}
	var ss []kv
	for k, v := range config.HostBytes {
		ss = append(ss, kv{k, v})
	}
	sort.Slice(ss, func(i, j int) bool { return ss[i].Value > ss[j].Value })

	top5 := ss
	if len(top5) > 5 {
		top5 = top5[:5]
	}
	totalB := config.TotalBytes
	config.BytesMutex.Unlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	uptimeDuration := time.Since(config.StartTime)
	hours := int(uptimeDuration.Hours())
	minutes := int(uptimeDuration.Minutes()) % 60
	seconds := int(uptimeDuration.Seconds()) % 60
	uptimeStr := fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)

	aiLearning := time.Since(config.LastLearning) < 5*time.Second

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_bytes":     totalB,
		"latency":         config.LastLatency,
		"top_talkers":     top5,
		"uptime":          uptimeStr,
		"active_conns":    config.ActiveConns,
		"mem_alloc_bytes": m.Alloc,
		"ads_blocked":     config.AdsBlocked,
		"vip_allowed":     config.VIPAllowed,
		"tele_hits":       config.TeleHits,
		"cache_hits":      config.CacheHits,
		"ai_learning":     aiLearning,
	})
}

// BALIKIN FUNGSI YANG KEPOTONG:
func handleLogs(w http.ResponseWriter, r *http.Request) {
	config.LogMutex.Lock()
	defer config.LogMutex.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if config.RealTrafficLogs == nil {
		json.NewEncoder(w).Encode([]config.TrafficLog{})
	} else {
		json.NewEncoder(w).Encode(config.RealTrafficLogs)
	}
}

func handleResetAI(w http.ResponseWriter, r *http.Request) {
	config.LastLearning = time.Now() // Trigger animasi web
	w.WriteHeader(http.StatusOK)
}

type ActionReq struct {
	Target string `json:"target"`
	Action string `json:"action"`
}

func handleAction(w http.ResponseWriter, r *http.Request) {
	var req ActionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
		if req.Action == "REGULAR" {
			config.RulesMutex.Lock()
			delete(config.DynamicRules, req.Target)
			config.RulesMutex.Unlock()
		} else {
			logic.AddDynamicRule(req.Target, req.Action)
		}
	}
	w.WriteHeader(http.StatusOK)
}
