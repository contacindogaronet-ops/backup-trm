package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"nganjuk-engine-reborn/config"
	"nganjuk-engine-reborn/dashboard"
	"nganjuk-engine-reborn/gateway"
)

//go:embed neural-engine
var embeddedNeuralBin []byte

func main() {
	fmt.Println("======================================================")
	fmt.Println("👑 NGANJUK EXTREME NANOSERVICE - AUTO DEPLOYMENT READY")
	fmt.Println("======================================================")

	// 1. MEKANISME INCEPTION (EKSTRAKSI BINER AI OTOMATIS)
	fmt.Println("[SYSTEM] 🧪 Melepaskan biner Neural Engine dari dalam perut proxy...")
	
	// Tulis paksa biner AI dengan permission Execute (0755)
	err := os.WriteFile("./neural-engine", embeddedNeuralBin, 0755)
	if err != nil {
		fmt.Printf("[FATAL] Gagal mengekstrak biner AI: %v\n", err)
		return
	}
	
	cmd := exec.Command("./neural-engine")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Println("[SYSTEM] 🧠 Meluncurkan AI Daemon (Port 8877)...")
	
	// Jika file otak ENTERPRISE_BRAIN_50NODES.bin tidak ada, 
	// daemon ini akan OTOMATIS membuatnya saat Start!
	if err := cmd.Start(); err != nil {
		fmt.Printf("[FATAL] AI Daemon Gagal Start: %v\n", err)
		return
	}

	time.Sleep(500 * time.Millisecond) // Jeda nafas agar AI siap sedia

	// 2. JALANKAN INFRASTRUKTUR PROXY & WEB
	fmt.Printf("[SYSTEM] 🌐 HUD SPA Aktif di http://127.0.0.1%s\n", config.WebPort)
	go dashboard.StartC2CServer(config.WebPort)

	fmt.Printf("[SYSTEM] ⚡ Universal TCP/UDP SOCKS5 Gate Listening di %s\n", config.ProxyPort)
	go gateway.StartTCPServer(config.ProxyPort)

	// 3. ELEGANT SHUTDOWN
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	if cmd.Process != nil { cmd.Process.Signal(syscall.SIGTERM) }
	fmt.Println("\n[SYSTEM] 🛑 Semua Nanoservice Mati Paripurna!")
}
