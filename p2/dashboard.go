package main

import (
	"net"
	"strconv"
	"time"

	"github.com/rs/zerolog"
)

type DashboardServer struct {
	addr      string
	telemetry *TelemetryTracker
	zlog      zerolog.Logger
	state     *Engine
}

func NewDashboardServer(addr string, t *TelemetryTracker, engine *Engine, logger zerolog.Logger) *DashboardServer {
	return &DashboardServer{
		addr:      addr,
		telemetry: t,
		zlog:      logger,
		state:     engine,
	}
}

func (d *DashboardServer) Start() {
	l, err := net.Listen("tcp", d.addr)
	if err != nil {
		d.zlog.Error().Err(err).Str("addr", d.addr).Msg("dashboard listener failed")
		return
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		go d.handleTelemetryHTTP(conn)
	}
}

func (d *DashboardServer) handleTelemetryHTTP(c net.Conn) {
	defer c.Close()

	buf := make([]byte, 1024)
	_, _ = c.Read(buf)

	uptime := uint64(time.Since(d.telemetry.StartTime).Seconds())
	activeConns := d.telemetry.ActiveConns.Load()
	allowed := d.telemetry.TotalAllowed.Load()
	blocked := d.telemetry.TotalBlocked.Load()
	rules := d.telemetry.TotalLoadedRules.Load()
	traffic := d.telemetry.BytesTransferred.Load()
	proxyState := *d.state.state.Load()

	jsonBuf := make([]byte, 0, 512)
	jsonBuf = append(jsonBuf, `{"proxy_state":"`...)
	jsonBuf = append(jsonBuf, proxyState...)
	jsonBuf = append(jsonBuf, `","uptime_sec":`...)
	jsonBuf = strconv.AppendUint(jsonBuf, uptime, 10)
	jsonBuf = append(jsonBuf, `,"active_connections":`...)
	jsonBuf = strconv.AppendInt(jsonBuf, activeConns, 10)
	jsonBuf = append(jsonBuf, `,"total_allowed":`...)
	jsonBuf = strconv.AppendUint(jsonBuf, allowed, 10)
	jsonBuf = append(jsonBuf, `,"total_blocked":`...)
	jsonBuf = strconv.AppendUint(jsonBuf, blocked, 10)
	jsonBuf = append(jsonBuf, `,"total_rules":`...)
	jsonBuf = strconv.AppendUint(jsonBuf, rules, 10)
	jsonBuf = append(jsonBuf, `,"bytes_transferred":`...)
	jsonBuf = strconv.AppendUint(jsonBuf, traffic, 10)
	jsonBuf = append(jsonBuf, `}`...)

	resp := make([]byte, 0, 512)
	resp = append(resp, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nConnection: close\r\nContent-Length: "...)
	resp = strconv.AppendInt(resp, int64(len(jsonBuf)), 10)
	resp = append(resp, "\r\n\r\n"...)
	resp = append(resp, jsonBuf...)

	_, _ = c.Write(resp)
}
