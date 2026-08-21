package main

import (
	"encoding/json"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/rs/zerolog"
)

type StateFile struct {
	ProxyState string `json:"proxy_state"`
}

func loadState(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "ON"
	}
	var sf StateFile
	if err := json.Unmarshal(b, &sf); err != nil {
		return "ON"
	}
	if sf.ProxyState == "OFF" {
		return "OFF"
	}
	return "ON"
}

func saveState(path string, state string) {
	sf := StateFile{ProxyState: state}
	b, err := json.MarshalIndent(sf, "", "  ")
	if err == nil {
		_ = os.WriteFile(path, b, 0644)
	}
}

func sanitizeFlagValue(val string) string {
	if strings.HasPrefix(val, "-") {
		return ""
	}
	return strings.TrimSpace(val)
}

func main() {
	flagListen := flag.String("L", "127.0.0.3:2007", "Local proxy listen address")
	flagState := flag.String("E", "", "Set proxy state immediately: ON | OFF")
	flagAddRule := flag.String("A", "", "Add allowed rule (IP/CIDR or domain)")
	flagBlockRule := flag.String("B", "", "Add blocked rule (IP/CIDR or domain)")
	flagResetRules := flag.Bool("R", false, "Reset rules cache")
	flagUDP := flag.Bool("U", false, "Enable UDP Associate handler")
	flagWeb := flag.Bool("W", false, "Enable Telemetry Dashboard on 127.0.0.3:2008")
	flagDaemon := flag.Bool("D", false, "Run in background daemon mode")
	flagRunMenu := flag.Bool("RUN", false, "Launch visual ANSI interactive terminal UI")
	flag.Parse()

	logFile, err := os.OpenFile("jargo.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	var zlog zerolog.Logger
	if err != nil {
		zlog = zerolog.New(os.Stderr).With().Timestamp().Logger()
	} else {
		defer logFile.Close()
		zlog = zerolog.New(logFile).With().Timestamp().Logger()
	}

	listenAddr := sanitizeFlagValue(*flagListen)
	if listenAddr == "" || !strings.Contains(listenAddr, ":") {
		listenAddr = "127.0.0.3:2007"
	}

	statePath := "state.json"
	persistedState := loadState(statePath)

	stateInput := sanitizeFlagValue(*flagState)
	if stateInput == "ON" || stateInput == "OFF" {
		persistedState = stateInput
		saveState(statePath, persistedState)
	}

	cfg := Config{
		EngineAddr:    listenAddr,
		DashboardAddr: "127.0.0.3:2008",
		EnableUDP:     *flagUDP,
		EnableWeb:     *flagWeb,
		StateFilePath: statePath,
	}

	engine, err := NewEngine(cfg, zlog)
	if err != nil {
		zlog.Fatal().Err(err).Msg("failed to initialize JARGO engine")
		return
	}
	engine.state.Store(&persistedState)

	if *flagResetRules {
		engine.db.Reset()
	}

	addRule := sanitizeFlagValue(*flagAddRule)
	if addRule != "" {
		if err := engine.rules.AddIPRule(addRule, CodeVIP); err != nil {
			engine.rules.AddDomainRule(addRule, CodeVIP)
		}
	}

	blockRule := sanitizeFlagValue(*flagBlockRule)
	if blockRule != "" {
		if err := engine.rules.AddIPRule(blockRule, CodeBlock); err != nil {
			engine.rules.AddDomainRule(blockRule, CodeBlock)
		}
	}

	if cfg.EnableWeb {
		dash := NewDashboardServer(cfg.DashboardAddr, engine.telemetry, engine, zlog)
		go dash.Start()
	}

	go func() {
		if err := engine.Start(); err != nil {
			zlog.Error().Err(err).Msg("engine listener stopped")
		}
	}()

	if *flagRunMenu && !*flagDaemon {
		go RenderCLIMenu(engine)
		go StartInteractiveCLI(engine) // Jalankan listener command terminal
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	engine.Stop()
	saveState(statePath, *engine.state.Load())
	_, _ = os.Stdout.WriteString("\n[JARGO ENGINE] Clean shutdown completed.\n")
}
