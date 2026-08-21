package dashboard

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
)

// 🛡️ FIX EMBED: Kembali normal, cuma baca root folder hud/ aja!
//go:embed pages/hud/*
var hudEmbed embed.FS

func StartC2CServer(addr string) {
	http.HandleFunc("/api/stats", handleStats)
	http.HandleFunc("/api/logs", handleLogs)
	http.HandleFunc("/api/reset_ai", handleResetAI)
	http.HandleFunc("/api/action", handleAction)

	// Buka akses ke folder root
	hudRoot, err := fs.Sub(hudEmbed, "pages/hud")
	if err != nil { fmt.Printf("[FATAL] FS error: %v\n", err); return }

	http.Handle("/", http.FileServer(http.FS(hudRoot)))
	fmt.Printf("[SYSTEM] 🌐 HUD MPA Aktif di http://127.0.0.1%s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil { fmt.Printf("[FATAL] UI down: %v\n", err) }
}
