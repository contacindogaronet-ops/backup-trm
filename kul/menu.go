package main

import (
	"bufio"
	"os"
	"strings"
)

// ==========================================
// 🎨 ANSI COLOR CODES (ZERO-ALLOC UI)
// ==========================================
const (
	ClearScr = "\x1b[H\x1b[2J" // Membersihkan terminal
	Reset    = "\x1b[0m"
	Bold     = "\x1b[1m"
	Red      = "\x1b[1;31m"
	Green    = "\x1b[1;32m"
	Yellow   = "\x1b[1;33m"
	Cyan     = "\x1b[1;36m"
)

// ==========================================
// 🖥️ TACTICAL CLI MENU (UI MODULE)
// ==========================================
func showMenu(state *EngineState) {
	reader := bufio.NewReader(os.Stdin)

	for {
		// 1. Bersihkan layar setiap kali menu di-render (Anti-Spam/Kotor)
		os.Stdout.WriteString(ClearScr)

		// 2. Render Header
		os.Stdout.WriteString(Cyan + "╔═════════════════════════════════════════════════╗\n")
		os.Stdout.WriteString("║" + Bold + "      ⚡ PROJECT 2007 : MULTIPLEXER ENGINE      " + Reset + Cyan + "║\n")
		os.Stdout.WriteString("╚═════════════════════════════════════════════════╝\n" + Reset)

		// 3. Render Status Interaktif
		os.Stdout.WriteString("\n" + Bold + "  📡 STATUS ENGINE : ")
		if state.ProxyState == "ON" {
			os.Stdout.WriteString(Green + "[ 🟢 VLESS TUNNEL AKTIF ]\n" + Reset)
		} else {
			os.Stdout.WriteString(Red + "[ 🔴 DIRECT LOCAL AKTIF ]\n" + Reset)
		}
		os.Stdout.WriteString(Cyan + " ─────────────────────────────────────────────────\n" + Reset)

		// 4. Render Opsi Menu
		os.Stdout.WriteString(Bold + "  [1] " + Reset + "Nyalakan Proxy Tunnel " + Green + "(ON)\n" + Reset)
		os.Stdout.WriteString(Bold + "  [2] " + Reset + "Matikan Proxy " + Red + "(OFF - Direct Local)\n" + Reset)
		os.Stdout.WriteString(Bold + "  [3] " + Reset + Yellow + "🚀 JALANKAN ENGINE SEKARANG\n" + Reset)
		os.Stdout.WriteString(Bold + "  [0] " + Reset + "Keluar dari Sistem\n\n")

		// 5. Input Prompt
		os.Stdout.WriteString(Cyan + "  ❯ Pilih Eksekusi : " + Reset)

		// Baca Input
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		// Evaluasi Input
		switch input {
		case "1":
			state.ProxyState = "ON"
			saveState(*state)
			// Layar otomatis terhapus dan merender warna Hijau di loop berikutnya
		case "2":
			state.ProxyState = "OFF"
			saveState(*state)
			// Layar otomatis terhapus dan merender warna Merah di loop berikutnya
		case "3":
			// Bersihkan layar sebelum booting mesin agar log engine tampil rapi
			os.Stdout.WriteString(ClearScr)
			runCoreSystem(*state)
			return
		case "0":
			os.Stdout.WriteString(ClearScr + Red + "💀 Sistem dimatikan. Goodbye, Komandan.\n" + Reset)
			os.Exit(0)
		default:
			// Bypass input salah, otomatis mengulang loop
		}
	}
}
