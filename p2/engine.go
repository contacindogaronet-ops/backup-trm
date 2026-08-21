package main

import (
	"net"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/panjf2000/ants/v2"
	"github.com/rs/zerolog"
)

func NewEngine(cfg Config, logger zerolog.Logger) (*Engine, error) {
	db := fastcache.New(64 * 1024 * 1024)
	t := &TelemetryTracker{
		StartTime: time.Now(),
	}

	p, err := ants.NewPool(10000, ants.WithNonblocking(false))
	if err != nil {
		return nil, err
	}

	rm := NewRuleManager(db, t, logger)

	eng := &Engine{
		cfg:       cfg,
		pool:      p,
		db:        db,
		rules:     rm,
		telemetry: t,
		activity:  &ActivityRing{},
		zlog:      logger,
	}
	initialState := "ON"
	eng.state.Store(&initialState)

	eng.rules.LoadRulesFromDirectory("~/.rules")
	totalRules := eng.telemetry.TotalLoadedRules.Load()
	eng.activity.Push(AnsiCyan + "[RULES] " + AnsiWhite + "Loaded " + strconv.FormatUint(totalRules, 10) + " rules" + AnsiReset)

	return eng, nil
}

func (e *Engine) Start() error {
	var err error
	e.listener, err = net.Listen("tcp", e.cfg.EngineAddr)
	if err != nil {
		return err
	}

	e.zlog.Info().Str("listen", e.cfg.EngineAddr).Msg("JARGO engine TCP listener started")
	e.activity.Push(AnsiGreen + "[START] " + AnsiWhite + "Bound on " + e.cfg.EngineAddr + AnsiReset)

	udpHandler := NewUDPRelayHandler(e.zlog, e.telemetry)

	for {
		conn, err := e.listener.Accept()
		if err != nil {
			return err
		}

		err = e.pool.Submit(func() {
			e.handleConnection(conn, udpHandler)
		})
		if err != nil {
			_ = conn.Close()
		}
	}
}

func (e *Engine) handleConnection(clientConn net.Conn, udp *UDPRelayHandler) {
	currentState := *e.state.Load() == "ON"
	connID := e.telemetry.ConnCounter.Add(1)
	connIDStr := "#ID-" + strconv.FormatUint(connID, 10)

	req, err := HandshakeSOCKS5(clientConn, e.rules, currentState)

	if err != nil {
		if err == errBlockedByRule && req != nil {
			e.telemetry.TotalBlocked.Add(1)
			blockedTarget := FormatAddrPort(req.DestAddr, req.DestPort)
			e.activity.Push(AnsiRed + "[BLOCK] " + AnsiMagenta + connIDStr + " " + AnsiWhite + blockedTarget + AnsiReset)
			e.zlog.Warn().Uint64("id", connID).Str("target", blockedTarget).Msg("target blocked by rule")
		}
		_ = clientConn.Close()
		return
	}

	if req.Cmd == CmdUDPAssociate {
		if !e.cfg.EnableUDP {
			_ = clientConn.Close()
			return
		}
		targetStr := FormatAddrPort(req.DestAddr, req.DestPort)
		e.activity.Push(AnsiYellow + "[UDP]   " + AnsiMagenta + connIDStr + " " + AnsiWhite + targetStr + AnsiReset)
		udp.HandleUDPAssociate(clientConn, req)
		return
	}

	dest := FormatAddrPort(req.DestAddr, req.DestPort)
	dialer := net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	// Mulai Pengukuran Waktu RTT Handshake
	startDial := time.Now()

	targetConn, err := dialer.Dial("tcp4", dest)
	if err != nil {
		targetConn, err = dialer.Dial("tcp", dest)
		if err != nil {
			_, _ = clientConn.Write([]byte{SocksVersion5, 0x04, 0x00, AtypIPv4, 0, 0, 0, 0, 0, 0})
			_ = clientConn.Close()
			return
		}
	}

	dialTookMs := time.Since(startDial).Milliseconds()

	if err := SendSocksSuccess(clientConn); err != nil {
		_ = clientConn.Close()
		_ = targetConn.Close()
		return
	}

	e.telemetry.TotalAllowed.Add(1)
	e.telemetry.ActiveConns.Add(1)

	tierColor := AnsiCyan
	if req.Tier == CodeVVIP {
		tierColor = AnsiYellow + AnsiBold
	}

	// Format Log Mirip V2Ray: [ALLOW] #ID-101 domain:port (TIER) [took 12ms]
	tookStr := strconv.FormatInt(dialTookMs, 10) + "ms"
	logMsg := AnsiGreen + "[ALLOW] " + AnsiMagenta + connIDStr + " " + AnsiWhite + dest + " " + tierColor + "(" + TierToString(req.Tier) + ")" + AnsiCyan + " [took " + tookStr + "]" + AnsiReset
	e.activity.Push(logMsg)

	e.zlog.Info().
		Uint64("id", connID).
		Str("target", dest).
		Str("tier", TierToString(req.Tier)).
		Int64("took_ms", dialTookMs).
		Msg("connection established")

	go func() {
		defer e.telemetry.ActiveConns.Add(-1)
		RelayTCP(clientConn, targetConn, req.Tier, e.telemetry)
	}()
}

func (e *Engine) Stop() {
	if e.listener != nil {
		_ = e.listener.Close()
	}
	e.pool.Release()
	e.db.Reset()
}
