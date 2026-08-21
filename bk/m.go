package main

import (
	"net"
	"os"
	"sync/atomic"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/joho/godotenv"
	"github.com/panjf2000/ants/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// ==========================================
// KONSTANTA & KASTA
// ==========================================
const (
	CodeBlock   byte = 0
	CodeReguler byte = 1
	CodeVIP     byte = 2
	CodeVVIP    byte = 3
)

// ==========================================
// STRUKTUR DATA UTAMA
// ==========================================
type TrieNode struct {
	Children [2]*TrieNode
	Tier     byte
	HasRule  bool
}

type voidWriter struct{}
func (v voidWriter) Write(p []byte) (int, error) { return len(p), nil }

// ==========================================
// VARIABEL GLOBAL (STATE)
// ==========================================
var (
	DBEngine *fastcache.Cache
	GPool    *ants.Pool
	ProxyIP  net.IP

	IPv4Trie = &TrieNode{}
	IPv6Trie = &TrieNode{}

	TotalAllowed     atomic.Uint64
	TotalBlocked     atomic.Uint64
	ActiveConns      atomic.Int64
	TotalLoadedRules atomic.Uint64

	StrictMode   bool
	EnableLog    bool
	LogErrors    bool
	LogAllow     bool
	LogBlock     bool
	LogReguler   bool
	LogUniversal bool
	EnableWebAPI bool
)

// ==========================================
// BOOTSTRAP (INISIALISASI CLI & WARNA)
// ==========================================
func init() {
	for _, arg := range os.Args {
		switch arg {
		case "telegram": StrictMode = true
		case "-L": EnableLog = true
		case "-E": LogErrors = true
		case "-A": LogAllow = true
		case "-B": LogBlock = true
		case "-R": LogReguler = true
		case "-U": LogUniversal = true
		case "-W": EnableWebAPI = true
		}
	}

	if EnableLog {
		output := zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: "15:04:05.000", // Presisi Milidetik
			NoColor:    false,          // Paksa output berwarna di Termux
		}

		output.FormatLevel = func(i interface{}) string {
			if ll, ok := i.(string); ok {
				switch ll {
				case "info": return "\x1b[1;32m[ OK ]\x1b[0m"
				case "warn": return "\x1b[1;33m[BLOK]\x1b[0m"
				case "error": return "\x1b[1;31m[FAIL]\x1b[0m"
				case "fatal": return "\x1b[1;41m[DEAD]\x1b[0m"
				}
			}
			return "[LOG]"
		}

		log.Logger = zerolog.New(output).With().Timestamp().Logger()
	} else {
		log.Logger = zerolog.New(voidWriter{})
	}
}

// ==========================================
// JANTUNG MESIN (GOROUTINE & CACHE)
// ==========================================
func InitCore() {
	// 1. Fastcache (64MB) untuk AdBlock Rules
	DBEngine = fastcache.New(64 * 1024 * 1024)

	// 2. Goroutine Pool (10.000 Worker) untuk mencegah OOM / HP Ngehang
	var err error
	GPool, err = ants.NewPool(10000)
	if err != nil {
		panic("🔥 FATAL: Gagal menyalakan Goroutine Pool! " + err.Error())
	}
}

// ==========================================
// HELPER: BACA ENVIRONMENT
// ==========================================
func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

// ==========================================
// FUNGSI UTAMA (ENTRYPOINT)
// ==========================================
func main() {
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

	// 1. Hidupkan Jantung Mesin
	InitCore()

	// 2. Tarik Rules (Pastikan fungsi ini ada di file rules.go lu)
	FetchRulesFromGithub()

	// 3. Jalankan Web API (Kalau diaktifkan pakai flag -W)
	if EnableWebAPI {
		if LogUniversal || LogAllow {
			log.Info().Msg("🌐 Web API Dashboard AKTIF di http://" + dashAddr)
		}
		// Pastikan StartDashboard ada di file dashboard lu
		go StartDashboard(dashAddr)
	}

	if EnableLog {
		log.Info().Msg("🔥 ENGINE AKTIF di " + engineAddr)
	}
	
	// 4. Buka Gerbang TCP/UDP (Fungsi ini ada di engine.go lu)
	StartEngine(engineAddr)
}
