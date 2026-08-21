package main

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	AnsiHome      = "\033[H"
	AnsiClearLine = "\033[2K"
	AnsiReset     = "\033[0m"
	AnsiBold      = "\033[1m"
	AnsiRed       = "\033[31m"
	AnsiGreen     = "\033[32m"
	AnsiYellow    = "\033[33m"
	AnsiBlue      = "\033[34m"
	AnsiCyan      = "\033[36m"
	AnsiWhite     = "\033[37m"
	AnsiMagenta   = "\033[35m"
	AnsiGray      = "\033[90m"
)

func formatBytesCompact(bytes uint64) string {
	if bytes < 1024*1024 {
		return strconv.FormatUint(bytes/1024, 10) + " KB"
	} else if bytes < 1024*1024*1024 {
		mb := float64(bytes) / (1024 * 1024)
		buf := make([]byte, 0, 16)
		buf = strconv.AppendFloat(buf, mb, 'f', 1, 64)
		buf = append(buf, " MB"...)
		return string(buf)
	}
	gb := float64(bytes) / (1024 * 1024 * 1024)
	buf := make([]byte, 0, 16)
	buf = strconv.AppendFloat(buf, gb, 'f', 2, 64)
	buf = append(buf, " GB"...)
	return string(buf)
}

// RenderCLIMenu merender HUD 11 baris yang muat di layar Termux saat keyboard aktif
func RenderCLIMenu(engine *Engine) {
	var mem runtime.MemStats
	var lastBytes uint64 = 0

	// Clear layar satu kali saat start
	_, _ = os.Stdout.WriteString("\033[2J\033[H")

	for {
		runtime.ReadMemStats(&mem)
		state := *engine.state.Load()

		currentBytes := engine.telemetry.BytesTransferred.Load()
		speedBps := currentBytes - lastBytes
		lastBytes = currentBytes

		if speedBps > engine.telemetry.PeakSpeedBps.Load() {
			engine.telemetry.PeakSpeedBps.Store(speedBps)
		}
		peakMBps := float64(engine.telemetry.PeakSpeedBps.Load()) / (1024 * 1024)
		speedMBps := float64(speedBps) / (1024 * 1024)

		speedStr := strconv.FormatFloat(speedMBps, 'f', 2, 64) + " MB/s"
		peakStr := strconv.FormatFloat(peakMBps, 'f', 2, 64) + " MB/s"

		buf := make([]byte, 0, 2048)
		buf = append(buf, AnsiHome...)

		// Baris 1: Header
		buf = append(buf, AnsiClearLine+AnsiCyan+AnsiBold+"┌─[ JARGO ENGINE :: 127.0.0.3:2007 ]─["+AnsiReset...)
		if state == "ON" {
			buf = append(buf, AnsiGreen+AnsiBold+"STATUS: ACTIVE (ON)"+AnsiCyan+" ]─┐\n"+AnsiReset...)
		} else {
			buf = append(buf, AnsiRed+AnsiBold+"STATUS: OFF"+AnsiCyan+" ]─────────┐\n"+AnsiReset...)
		}

		// Baris 2: Speed & Traffic
		buf = append(buf, AnsiClearLine+AnsiCyan+"│ "+AnsiWhite+"Speed: "+AnsiYellow+AnsiBold+speedStr+AnsiReset+AnsiWhite+" (Peak: "+AnsiMagenta+peakStr+AnsiReset+AnsiWhite+") | Traffic: "+AnsiGreen+formatBytesCompact(currentBytes)+AnsiReset+AnsiCyan+" │\n"+AnsiReset...)

		// Baris 3: Conns, Rules & RAM
		activeStr := strconv.FormatInt(engine.telemetry.ActiveConns.Load(), 10)
		allowStr := strconv.FormatUint(engine.telemetry.TotalAllowed.Load(), 10)
		blockStr := strconv.FormatUint(engine.telemetry.TotalBlocked.Load(), 10)
		rulesStr := strconv.FormatUint(engine.telemetry.TotalLoadedRules.Load(), 10)
		ramStr := strconv.FormatUint(mem.Alloc/1024, 10) + " KB"

		buf = append(buf, AnsiClearLine+AnsiCyan+"│ "+AnsiWhite+"Conns: "+AnsiGreen+activeStr+AnsiReset+AnsiWhite+" (OK:"+AnsiGreen+allowStr+AnsiReset+AnsiWhite+" | BLK:"+AnsiRed+blockStr+AnsiReset+AnsiWhite+") | Rules: "+AnsiYellow+rulesStr+AnsiReset+AnsiWhite+" | RAM: "+AnsiCyan+ramStr+AnsiReset+AnsiCyan+" │\n"+AnsiReset...)

		// Baris 4: Header Logs
		buf = append(buf, AnsiClearLine+AnsiCyan+"├──────────────────[ LIVE TRAFFIC LOGS ]────────────────────┤\n"+AnsiReset...)

		// Baris 5-7: 3 Log Terbaru
		recent := engine.activity.GetRecent()
		hasLog := false
		for i := 3; i < len(recent); i++ {
			line := recent[i]
			if line != "" {
				buf = append(buf, AnsiClearLine+" "+line+"\n"...)
				hasLog = true
			}
		}
		if !hasLog {
			buf = append(buf, AnsiClearLine+AnsiGray+" (waiting for incoming traffic...)\n"+AnsiReset...)
			buf = append(buf, AnsiClearLine+" \n"...)
			buf = append(buf, AnsiClearLine+" \n"...)
		}

		// Baris 8-11: Visual Command Box & Prompt
		buf = append(buf, AnsiClearLine+AnsiCyan+"├──────────────────[ INTERACTIVE CMD BOX ]──────────────────┤\n"+AnsiReset...)
		buf = append(buf, AnsiClearLine+AnsiYellow+AnsiBold+"│ "+AnsiRed+"0 <target>"+AnsiWhite+"=BLOCK  "+AnsiGreen+"1 <target>"+AnsiWhite+"=REGULER  "+AnsiCyan+"2 <target>"+AnsiWhite+"=VIP   "+AnsiYellow+"3 <target>"+AnsiWhite+"=VVIP │\n"+AnsiReset...)
		buf = append(buf, AnsiClearLine+AnsiWhite+"│ "+AnsiGreen+"on"+AnsiWhite+"/"+AnsiRed+"off"+AnsiWhite+"=State   "+AnsiMagenta+"cls"+AnsiWhite+"=Clear Log "+AnsiYellow+"reset"+AnsiWhite+"=Reset Rules                │\n"+AnsiReset...)
		buf = append(buf, AnsiClearLine+AnsiCyan+"└───────────────────────────────────────────────────────────┘\n"+AnsiReset...)
		buf = append(buf, AnsiClearLine+AnsiYellow+AnsiBold+"CMD >> "+AnsiReset...)

		_, _ = os.Stdout.WriteString(string(buf))
		time.Sleep(1 * time.Second)
	}
}

// StartInteractiveCLI membaca input perintah tanpa terganggu background loop
func StartInteractiveCLI(engine *Engine) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		parts := strings.Fields(text)
		if len(parts) == 1 {
			cmd := strings.ToLower(parts[0])
			switch cmd {
			case "on", "off":
				s := strings.ToUpper(cmd)
				engine.state.Store(&s)
				saveState(engine.cfg.StateFilePath, s)
				engine.activity.Push(AnsiCyan + "[STATE] " + AnsiWhite + "Proxy set to " + s + AnsiReset)
			case "cls", "clear":
				engine.activity.mu.Lock()
				engine.activity.lines = [6]string{}
				engine.activity.idx = 0
				engine.activity.mu.Unlock()
				engine.activity.Push(AnsiGray + "[LOGS] Activity buffer cleared" + AnsiReset)
			case "reset":
				engine.db.Reset()
				engine.activity.Push(AnsiYellow + "[RESET] Rule database cache cleared" + AnsiReset)
			default:
				engine.activity.Push(AnsiRed + "[ERR] Unknown command. Use: 0/1/2/3 <target>, on, off, cls" + AnsiReset)
			}
			continue
		}

		action := strings.ToLower(parts[0])
		target := parts[1]

		if idx := strings.IndexByte(target, ':'); idx != -1 {
			target = target[:idx]
		}

		var tier byte
		var valid = true

		switch action {
		case "0", "block", "b":
			tier = CodeBlock
		case "1", "reguler", "reg", "r":
			tier = CodeReguler
		case "2", "vip", "v":
			tier = CodeVIP
		case "3", "vvip", "vv":
			tier = CodeVVIP
		default:
			valid = false
		}

		if !valid {
			engine.activity.Push(AnsiRed + "[ERR] Invalid tier. Use 0(Block), 1(Reguler), 2(VIP), 3(VVIP)" + AnsiReset)
			continue
		}

		if err := engine.rules.ForceSetIPRule(target, tier); err != nil {
			engine.rules.ForceSetDomainRule(target, tier)
		}

		color := AnsiGreen
		if tier == CodeBlock {
			color = AnsiRed
		} else if tier == CodeVVIP {
			color = AnsiYellow + AnsiBold
		} else if tier == CodeVIP {
			color = AnsiCyan
		}

		engine.activity.Push(color + "[RULE SET] " + AnsiWhite + target + " -> " + TierToString(tier) + AnsiReset)
	}
}
