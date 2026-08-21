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
	clientAddr := clientConn.RemoteAddr().String()

	req, err := HandshakeSOCKS5(clientConn, e.rules, currentState)
	if err != nil {
		if err == errBlockedByRule {
			e.telemetry.TotalBlocked.Add(1)
			e.activity.Push(AnsiRed + "[BLOCK] " + AnsiWhite + clientAddr + " -> Blocklist" + AnsiReset)
		}
		_ = clientConn.Close()
		return
	}

	if req.Cmd == CmdUDPAssociate {
		if !e.cfg.EnableUDP {
			_ = clientConn.Close()
			return
		}
		e.activity.Push(AnsiYellow + "[UDP]   " + AnsiWhite + clientAddr + " -> UDP Assoc" + AnsiReset)
		udp.HandleUDPAssociate(clientConn, req)
		return
	}

	dest := FormatAddrPort(req.DestAddr, req.DestPort)
	dialer := net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	targetConn, err := dialer.Dial("tcp", dest)
	if err != nil {
		_, _ = clientConn.Write([]byte{SocksVersion5, 0x04, 0x00, AtypIPv4, 0, 0, 0, 0, 0, 0})
		_ = clientConn.Close()
		return
	}

	if err := SendSocksSuccess(clientConn, 2007); err != nil {
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
	e.activity.Push(AnsiGreen + "[ALLOW] " + AnsiWhite + dest + " " + tierColor + "(" + TierToString(req.Tier) + ")" + AnsiReset)

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