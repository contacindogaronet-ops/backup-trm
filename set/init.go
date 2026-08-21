package main

import (
	"os"
	"time"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/panjf2000/ants/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/valyala/fasthttp"
)

// ==========================================
// INISIALISASI
// ==========================================
func InitCore() {
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "15:04:05",
	})

	var err error
	GPool, err = ants.NewPool(49000, ants.WithPreAlloc(true))
	if err != nil {
		log.Fatal().Err(err).Msg("Gagal init ants")
	}

	DBEngine = fastcache.New(512 * 1024 * 1024) // Naikkan memori cache ke 512MB buat nahan puluhan juta domain

	FastCli = &fasthttp.Client{
		ReadTimeout:         15 * time.Second, // Timeout dilamain dikit buat narik file gaban
		MaxIdleConnDuration: 10 * time.Second,
	}

	log.Info().Msg("Sistem Inti & Pekerja Pool Aktif")
}
