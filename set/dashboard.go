package main

import (
	"runtime"
	"strconv"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/rs/zerolog/log"
	"github.com/valyala/fasthttp"
)

// ==========================================
// ZERO-ALLOC DASHBOARD API
// ==========================================
func StartDashboard(addr string) {
	requestHandler := func(ctx *fasthttp.RequestCtx) {
		switch string(ctx.Path()) {
		case "/stats":
			var mem runtime.MemStats
			runtime.ReadMemStats(&mem)
			var cacheStats fastcache.Stats
			DBEngine.UpdateStats(&cacheStats)

			ctx.SetContentType("application/json")
			ctx.WriteString(`{`)
			ctx.WriteString(`"active_connections": `)
			ctx.WriteString(strconv.FormatInt(ActiveConns.Load(), 10))
			ctx.WriteString(`, "total_allowed": `)
			ctx.WriteString(strconv.FormatUint(TotalAllowed.Load(), 10))
			ctx.WriteString(`, "total_blocked": `)
			ctx.WriteString(strconv.FormatUint(TotalBlocked.Load(), 10))
			ctx.WriteString(`, "total_rules_db": `)
			ctx.WriteString(strconv.FormatUint(TotalLoadedRules.Load(), 10))
			ctx.WriteString(`, "ram_mb": `)
			ctx.WriteString(strconv.FormatUint(mem.Alloc/1024/1024, 10))
			ctx.WriteString(`, "pool_running": `)
			ctx.WriteString(strconv.Itoa(GPool.Running()))
			ctx.WriteString(`, "pool_cap": `)
			ctx.WriteString(strconv.Itoa(GPool.Cap()))
			ctx.WriteString(`, "cache_collisions": `)
			ctx.WriteString(strconv.FormatUint(cacheStats.Collisions, 10))
			ctx.WriteString(`}`)
		default:
			ctx.Error("Not Found", fasthttp.StatusNotFound)
		}
	}
	log.Info().Msgf("📊 REAL-TIME DASHBOARD ACTIVE AT http://%s/stats", addr)
	_ = fasthttp.ListenAndServe(addr, requestHandler)
}
