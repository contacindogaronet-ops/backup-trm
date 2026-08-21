package config

import (
	"sync"
	"time"
)

type TrafficLog struct {
	Time    string `json:"time"`
	Target  string `json:"target"`
	Verdict string `json:"verdict"`
}

// 💥 INI DIA BLUEPRINT YANG TADI GUE HAPUS BANGSAT!
type ClientStats struct {
	Hits       int
	LastActive time.Time
	BurstCount int
}

var (
	StartTime       = time.Now()
	LogMutex        sync.Mutex
	RealTrafficLogs []TrafficLog
	
	LastLearning     time.Time
	LastAnchorDating time.Time
	LastAnchorTele   time.Time

	// 💥 INI DIA MAP RADAR TCP LU YANG SEMPAT HILANG!
	StatsMap         = make(map[string]ClientStats)
	StatsMutex       sync.Mutex

	AdsBlocked  int
	VIPAllowed  int
	TeleHits    int
	DatingHits  int
	CacheHits   int
	ActiveConns int

	// Atomic Operation Variables
	TotalBytes  int64
	BytesMutex  sync.Mutex
	
	DynamicRules   = make(map[string]string)
	RulesMutex     sync.Mutex
	
	HostBytes      = make(map[string]int64)
	LastLatency    int64
)
