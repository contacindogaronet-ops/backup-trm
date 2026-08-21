package main

import (
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/joho/godotenv"
	"github.com/panjf2000/ants/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	CodeBlock   byte = 0
	CodeReguler byte = 1
	CodeVIP     byte = 2
	CodeVVIP    byte = 3

	BaseBuffer = 1 * 1024 * 1024 // 1MB Socket Buffer
)

type TrieNode struct {
	Children [2]*TrieNode
	Tier     byte
	HasRule  bool
}

// Custom Void Writer untuk menggantikan io.Discard (Bebas package io)
type voidWriter struct{}
func (v voidWriter) Write(p []byte) (int, error) { return len(p), nil }

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

	// 🔴 CLI FLAGS
	StrictMode   bool 
	EnableLog    bool 
	LogErrors    bool 
	LogAllow     bool 
	LogBlock     bool 
	LogReguler   bool 
	LogUniversal bool 
	EnableWebAPI bool 
)

func init() {
	for _, arg := range os.Args {
		switch arg {
		case "telegram":
			StrictMode = true
		case "-L":
			EnableLog = true
		case "-E":
			LogErrors = true
		case "-A":
			LogAllow = true
		case "-B":
			LogBlock = true
		case "-R":
			LogReguler = true
		case "-U":
			LogUniversal = true
		case "-W":
			EnableWebAPI = true
		}
	}

	if EnableLog {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		})
	} else {
		log.Logger = zerolog.New(voidWriter{}) // 0% CPU Overhead, 0 io package
	}
}

func main() {
	_ = godotenv.Load()

	virtualIP := getEnv("VIRTUAL_IP", "127.0.0.1")
	enginePort := getEnv("ENGINE_PORT", "2007")
	dashPort := getEnv("DASH_PORT", "2008")

	engineAddr := virtualIP + ":" + enginePort
	dashAddr := virtualIP + ":" + dashPort
	ProxyIP = net.ParseIP(virtualIP).To4()
	if ProxyIP == nil {
		ProxyIP = net.ParseIP("127.0.0.1").To4()
	}

	InitCore()
	FetchRulesFromGithub()

	if EnableWebAPI {
		if LogUniversal || LogAllow {
			log.Info().Msgf("🌐 Web API Dashboard AKTIF di http://%s", dashAddr)
		}
		go StartDashboard(dashAddr)
	}

	if EnableLog {
		log.Info().Msgf("🔥 ENGINE AKTIF di %s | Strict: %v", engineAddr, StrictMode)
	}
	StartEngine(engineAddr)
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
