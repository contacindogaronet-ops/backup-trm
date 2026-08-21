package main

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/panjf2000/ants/v2"
	"github.com/rs/zerolog"
)

// ANSI Terminal Colors & Formatting (Global Shared)
const (
	AnsiReset       = "\033[0m"
	AnsiBold        = "\033[1m"
	AnsiRed         = "\033[31m"
	AnsiGreen       = "\033[32m"
	AnsiYellow      = "\033[33m"
	AnsiBlue        = "\033[34m"
	AnsiMagenta     = "\033[35m"
	AnsiCyan        = "\033[36m"
	AnsiWhite       = "\033[37m"
	AnsiGray        = "\033[90m"
	AnsiHome        = "\033[H"
	AnsiClearLine   = "\033[2K"
	AnsiClearScreen = "\033[2J\033[H"
)

// Routing Tiers
const (
	CodeBlock   byte = 0
	CodeReguler byte = 1
	CodeVIP     byte = 2
	CodeVVIP    byte = 3
)

func TierToString(tier byte) string {
	switch tier {
	case CodeBlock:
		return "BLOCK"
	case CodeReguler:
		return "REGULER"
	case CodeVIP:
		return "VIP"
	case CodeVVIP:
		return "VVIP"
	default:
		return "UNKNOWN"
	}
}

// SOCKS5 Protocol Constants
const (
	SocksVersion5       byte = 0x05
	MethodNoAuth        byte = 0x00
	MethodNoAcceptable  byte = 0xFF
	CmdConnect          byte = 0x01
	CmdUDPAssociate     byte = 0x03
	AtypIPv4            byte = 0x01
	AtypDomain          byte = 0x03
	AtypIPv6            byte = 0x04
	RepSuccess          byte = 0x00
	RepRuleFailure      byte = 0x02
	RepCmdNotSupported  byte = 0x07
	RepAtypNotSupported byte = 0x08
)

type ActivityRing struct {
	mu    sync.Mutex
	lines [6]string
	idx   int
}

func (r *ActivityRing) Push(line string) {
	r.mu.Lock()
	r.lines[r.idx%6] = line
	r.idx++
	r.mu.Unlock()
}

func (r *ActivityRing) GetRecent() [6]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var res [6]string
	start := 0
	if r.idx > 6 {
		start = r.idx - 6
	}
	for i := 0; i < 6; i++ {
		curr := start + i
		if curr < r.idx {
			res[i] = r.lines[curr%6]
		}
	}
	return res
}

type TelemetryTracker struct {
	ActiveConns      atomic.Int64
	TotalAllowed     atomic.Uint64
	TotalBlocked     atomic.Uint64
	TotalLoadedRules atomic.Uint64
	BytesTransferred atomic.Uint64
	PeakSpeedBps     atomic.Uint64
	ConnCounter      atomic.Uint64 // Unique Connection ID Counter
	StartTime        time.Time
}

type Config struct {
	EngineAddr    string
	DashboardAddr string
	EnableUDP     bool
	EnableWeb     bool
	StateFilePath string
}

type Engine struct {
	cfg       Config
	listener  net.Listener
	pool      *ants.Pool
	db        *fastcache.Cache
	rules     *RuleManager
	telemetry *TelemetryTracker
	activity  *ActivityRing
	zlog      zerolog.Logger
	state     atomic.Pointer[string]
}
