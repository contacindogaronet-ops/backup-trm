package main

import (
	"net"
	"os"
	"sync/atomic"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/joho/godotenv"
	"github.com/panjf2000/ants/v2"
	"github.com/valyala/fasthttp"
)

// ==========================================
// KASTA & KONSTANTA LIMIT
// ==========================================
const (
	CodeBlock   byte = 0
	CodeReguler byte = 1
	CodeVIP     byte = 2
	CodeVVIP    byte = 3

	BaseBuffer = 6 * 1024 * 1024 // 4MB (Limit VVIP)
)

type IPRule struct {
	Net  *net.IPNet
	Tier byte
}

var (
	DBEngine *fastcache.Cache
	GPool    *ants.Pool
	FastCli  *fasthttp.Client
	ProxyIP  net.IP

	// Global Slice buat CIDR/IP murni
	IPRules []IPRule

	// Atomic Counters
	TotalAllowed     atomic.Uint64
	TotalBlocked     atomic.Uint64
	ActiveConns      atomic.Int64
	TotalLoadedRules atomic.Uint64

	// Mode Whitelist Ketat
	StrictMode bool
)

func main() {
	// Load config dari .env jika ada (Manajemen Virtual IP & Port)
	_ = godotenv.Load()

	virtualIP := getEnv("VIRTUAL_IP", "127.0.0.3")
	enginePort := getEnv("ENGINE_PORT", "2007")
	dashPort := getEnv("DASH_PORT", "2008")

	engineAddr := virtualIP + ":" + enginePort
	dashAddr := virtualIP + ":" + dashPort
	ProxyIP = net.ParseIP(virtualIP).To4()
	if ProxyIP == nil {
		ProxyIP = net.ParseIP("127.0.0.3").To4()
	}

	InitCore()

	// 🔴 AKTIVASI STRICT MODE (Contoh: ./engine telegram)
	if len(os.Args) > 1 && os.Args[1] == "telegram" {
		StrictMode = true
	}

	// Tarik jutaan IP/Domain secara sinkron di awal biar engine siap!
	FetchRulesFromGithub()

	go StartDashboard(dashAddr)
	StartEngine(engineAddr)
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
