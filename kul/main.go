package main

import (
	"encoding/json"
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
// STRUKTUR DATA UTAMA & ENGINE STATE
// ==========================================
type TrieNode struct {
	Children [2]*TrieNode
	Tier     byte
	HasRule  bool
}

// Hanya menyimpan status ON/OFF
type EngineState struct {
	ProxyState string `json:"proxy_state"` 
}

const configPath = "state.json"

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
	DebugMode    bool
	RunHeadless  bool

	GlobalProxyState string
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
		case "-D": DebugMode = true
		case "-RUN": RunHeadless = true
		}
	}

	if EnableLog {
		output := zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: "15:04:05.000",
			NoColor:    false,
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
	DBEngine = fastcache.New(64 * 1024 * 1024)
	var err error
	GPool, err = ants.NewPool(10000)
	if err != nil {
		panic("🔥 FATAL: Gagal menyalakan Goroutine Pool! " + err.Error())
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

// ==========================================
// STATE MANAGEMENT (JSON I/O)
// ==========================================
func loadState() EngineState {
	file, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return EngineState{ProxyState: "OFF"} // Default Mode
		}
		log.Fatal().Err(err).Msg("Gagal membaca state.json")
	}
	defer file.Close()

	var state EngineState
	if err := json.NewDecoder(file).Decode(&state); err != nil {
		log.Fatal().Err(err).Msg("Korup JSON terdeteksi pada state.json")
	}
	return state
}

func saveState(state EngineState) {
	file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Error().Err(err).Msg("Gagal membuka file config")
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.Encode(state)
}

// ==========================================
// CORE EXECUTION WRAPPER
// ==========================================
func runCoreSystem(state EngineState) {
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

	// Set global variable untuk relay.go / engine.go
	GlobalProxyState = state.ProxyState

	if EnableLog {
		if GlobalProxyState == "OFF" {
			log.Info().Msg("⚙️ TARGET ROUTING: DIRECT LOCAL (PROXY OFF)")
		} else {
			log.Info().Msg("🛡️ TARGET ROUTING: PROXY TUNNEL (PROXY ON)")
		}
	}

	InitCore()
	FetchRulesFromGithub()

	if EnableWebAPI {
		if LogUniversal || LogAllow {
			log.Info().Msg("🌐 Web API Dashboard AKTIF di http://" + dashAddr)
		}
		go StartDashboard(dashAddr)
	}

	if EnableLog {
		log.Info().Msg("🔥 ENGINE AKTIF di " + engineAddr)
	}
	
	StartEngine(engineAddr)
}

// ==========================================
// FUNGSI UTAMA (ENTRYPOINT)
// ==========================================
func main() {
	state := loadState()

	if RunHeadless {
		runCoreSystem(state)
	} else {
		showMenu(&state)
	}
}
