package main

import (
	"encoding/json"
	
	"github.com/rs/zerolog/log"
	"github.com/valyala/fasthttp"
)

func StartDashboard(addr string) {
	requestHandler := func(ctx *fasthttp.RequestCtx) {
		ctx.SetContentType("application/json")
		
		stats := map[string]interface{}{
			"status":       "online",
			"active_conns": ActiveConns.Load(),
			"allowed":      TotalAllowed.Load(),
			"blocked":      TotalBlocked.Load(),
			"total_rules":  TotalLoadedRules.Load(),
		}

		data, _ := json.Marshal(stats)
		ctx.SetBody(data)
	}

	server := &fasthttp.Server{
		Handler:            requestHandler,
		MaxConnsPerIP:      50,
		MaxRequestsPerConn: 100,
	}

	if err := server.ListenAndServe(addr); err != nil {
		log.Fatal().Err(err).Msg("🚨 Dashboard Web API gagal berjalan")
	}
}
