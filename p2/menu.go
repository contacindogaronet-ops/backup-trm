package main

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func formatBytesCompact(bytes uint64) string {
	if bytes < 1024*1024 {
		return strconv.FormatUint(bytes/1024, 10) + " KB"
	} else if bytes < 1024*1024*1024 {
		mb := float64(bytes) / (1024 * 1024)
		buf := make([]byte, 0, 16)
		buf = strconv.AppendFloat(buf, mb, 'f', 2, 64)
		buf = append(buf, " MB"...)
		return string(buf)
	}
	gb := float64(bytes) / (1024 * 1024 * 1024)
	buf := make([]byte, 0, 16)
	buf = strconv.AppendFloat(buf, gb, 'f', 2, 64)
	buf = append(buf, " GB"...)
	return string(buf)
}

func printBanner() {
	buf := make([]byte, 0, 1024)
	buf = append(buf, AnsiClearScreen...)
	buf = append(buf, AnsiCyan+AnsiBold...)
	buf = append(buf, "================================================================\n"...)
	buf = append(buf, "       JARGO ENGINE :: HIGH-PERFORMANCE SOCKS5 CONTROLLER       \n"...)
	buf = append(buf, "================================================================\n"+AnsiReset...)
	buf = append(buf, AnsiWhite+" [Engine] Bound: "+AnsiYellow+"127.0.0.3:2007"+AnsiWhite+" | [Web Dashboard] "+AnsiYellow+"http://127.0.0.3:2008"+AnsiReset+"\n"...)
	buf = append(buf, AnsiGray+" Type "+AnsiYellow+"'help'"+AnsiGray+" to view commands | Type "+AnsiYellow+"'stats'"+AnsiGray+" to see live metrics\n"+AnsiReset...)
	buf = append(buf, AnsiCyan+"----------------------------------------------------------------\n"+AnsiReset...)
	_, _ = os.Stdout.WriteString(string(buf))
}

func printStats(engine *Engine, lastBytes *uint64, lastTime *time.Time) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	now := time.Now()
	dur := now.Sub(*lastTime).Seconds()
	if dur <= 0 {
		dur = 1
	}

	currentBytes := engine.telemetry.BytesTransferred.Load()
	delta := currentBytes - *lastBytes
	*lastBytes = currentBytes
	*lastTime = now

	speedBps := uint64(float64(delta) / dur)
	if speedBps > engine.telemetry.PeakSpeedBps.Load() {
		engine.telemetry.PeakSpeedBps.Store(speedBps)
	}
	peakMBps := float64(engine.telemetry.PeakSpeedBps.Load()) / (1024 * 1024)
	speedMBps := float64(speedBps) / (1024 * 1024)

	uptime := uint64(time.Since(engine.telemetry.StartTime).Seconds())
	state := *engine.state.Load()

	buf := make([]byte, 0, 1024)
	buf = append(buf, "\n"+AnsiCyan+AnsiBold+"[--- ENGINE TELEMETRY SNAPSHOT ---]\n"+AnsiReset...)
	
	buf = append(buf, AnsiWhite+" Status        : "...)
	if state == "ON" {
		buf = append(buf, AnsiGreen+AnsiBold+"ACTIVE (ON)\n"+AnsiReset...)
	} else {
		buf = append(buf, AnsiRed+AnsiBold+"DISABLED (OFF)\n"+AnsiReset...)
	}

	buf = append(buf, AnsiWhite+" Current Speed : "+AnsiYellow+AnsiBold+strconv.FormatFloat(speedMBps, 'f', 2, 64)+" MB/s"+AnsiReset+AnsiWhite+" (Peak: "+AnsiMagenta+strconv.FormatFloat(peakMBps, 'f', 2, 64)+" MB/s)"+AnsiReset+"\n"...)
	buf = append(buf, AnsiWhite+" Total Traffic : "+AnsiGreen+formatBytesCompact(currentBytes)+AnsiReset+"\n"...)
	buf = append(buf, AnsiWhite+" Active Conns  : "+AnsiCyan+strconv.FormatInt(engine.telemetry.ActiveConns.Load(), 10)+AnsiReset+"\n"...)
	buf = append(buf, AnsiWhite+" Allowed/Block : "+AnsiGreen+strconv.FormatUint(engine.telemetry.TotalAllowed.Load(), 10)+AnsiReset+" / "+AnsiRed+strconv.FormatUint(engine.telemetry.TotalBlocked.Load(), 10)+AnsiReset+"\n"...)
	buf = append(buf, AnsiWhite+" Loaded Rules  : "+AnsiYellow+strconv.FormatUint(engine.telemetry.TotalLoadedRules.Load(), 10)+AnsiReset+"\n"...)
	buf = append(buf, AnsiWhite+" RAM Usage     : "+AnsiCyan+strconv.FormatUint(mem.Alloc/1024, 10)+" KB"+AnsiReset+"\n"...)
	buf = append(buf, AnsiWhite+" Engine Uptime : "+strconv.FormatUint(uptime, 10)+" sec\n"+AnsiReset...)
	buf = append(buf, AnsiCyan+"-----------------------------------\n"+AnsiReset...)

	_, _ = os.Stdout.WriteString(string(buf))
}

func printLogs(engine *Engine) {
	recent := engine.activity.GetRecent()
	buf := make([]byte, 0, 1024)
	buf = append(buf, "\n"+AnsiYellow+AnsiBold+"[--- RECENT ACTIVITY LOGS ---]\n"+AnsiReset...)
	hasLog := false
	for _, line := range recent {
		if line != "" {
			buf = append(buf, " "+line+"\n"...)
			hasLog = true
		}
	}
	if !hasLog {
		buf = append(buf, AnsiGray+" (no traffic logs yet)\n"+AnsiReset...)
	}
	buf = append(buf, AnsiYellow+"------------------------------\n"+AnsiReset...)
	_, _ = os.Stdout.WriteString(string(buf))
}

func printHelp() {
	buf := make([]byte, 0, 1024)
	buf = append(buf, "\n"+AnsiCyan+AnsiBold+"[--- JARGO COMMAND GUIDE ---]\n"+AnsiReset...)
	buf = append(buf, AnsiYellow+" 3 <target>"+AnsiWhite+"  or "+AnsiYellow+"vvip <target>"+AnsiWhite+"    : Set domain/IP to VVIP (Bypass & Max Speed)\n"...)
	buf = append(buf, AnsiCyan+" 2 <target>"+AnsiWhite+"  or "+AnsiCyan+"vip <target>"+AnsiWhite+"     : Set domain/IP to VIP\n"...)
	buf = append(buf, AnsiGreen+" 1 <target>"+AnsiWhite+"  or "+AnsiGreen+"reguler <target>"+AnsiWhite+" : Set domain/IP to REGULER\n"...)
	buf = append(buf, AnsiRed+" 0 <target>"+AnsiWhite+"  or "+AnsiRed+"block <target>"+AnsiWhite+"   : BLOCK domain/IP instantly\n"...)
	buf = append(buf, AnsiWhite+" stats      or s                : Show real-time speedometer & telemetry\n"...)
	buf = append(buf, AnsiWhite+" logs       or l                : Show recent allowed/blocked connections\n"...)
	buf = append(buf, AnsiWhite+" on / off                       : Switch proxy state ON or OFF\n"...)
	buf = append(buf, AnsiWhite+" reset                          : Clear fastcache rule database\n"...)
	buf = append(buf, AnsiWhite+" cls        or clear            : Clear terminal screen\n"...)
	buf = append(buf, AnsiWhite+" exit       or q                : Stop proxy and exit\n"...)
	buf = append(buf, AnsiCyan+"----------------------------\n"+AnsiReset...)
	_, _ = os.Stdout.WriteString(string(buf))
}

func StartInteractiveCLI(engine *Engine) {
	printBanner()

	var lastBytes uint64 = 0
	lastTime := time.Now()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		_, _ = os.Stdout.WriteString(AnsiCyan + AnsiBold + "jargo >> " + AnsiReset)
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		cmd := strings.ToLower(parts[0])

		if len(parts) == 1 {
			switch cmd {
			case "stats", "s", "status":
				printStats(engine, &lastBytes, &lastTime)
			case "logs", "l", "log":
				printLogs(engine)
			case "help", "?", "h":
				printHelp()
			case "cls", "clear":
				printBanner()
			case "on", "off":
				s := strings.ToUpper(cmd)
				engine.state.Store(&s)
				saveState(engine.cfg.StateFilePath, s)
				_, _ = os.Stdout.WriteString(AnsiGreen + "[SUCCESS] Proxy state changed to: " + s + "\n" + AnsiReset)
			case "reset":
				engine.db.Reset()
				_, _ = os.Stdout.WriteString(AnsiYellow + "[RESET] Rule cache database cleared.\n" + AnsiReset)
			case "exit", "quit", "q":
				_, _ = os.Stdout.WriteString(AnsiRed + "[SHUTDOWN] Stopping JARGO Engine...\n" + AnsiReset)
				engine.Stop()
				os.Exit(0)
			default:
				_, _ = os.Stdout.WriteString(AnsiRed + "[ERROR] Unknown command. Type 'help' for instructions.\n" + AnsiReset)
			}
			continue
		}

		action := cmd
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
			_, _ = os.Stdout.WriteString(AnsiRed + "[ERROR] Invalid tier. Use 0(Block), 1(Reguler), 2(VIP), 3(VVIP)\n" + AnsiReset)
			continue
		}

		if err := engine.rules.ForceSetIPRule(target, tier); err != nil {
			engine.rules.ForceSetDomainRule(target, tier)
		}

		tierName := TierToString(tier)
		color := AnsiGreen
		if tier == CodeBlock {
			color = AnsiRed
		} else if tier == CodeVVIP {
			color = AnsiYellow + AnsiBold
		}

		engine.activity.Push(color + "[CMD RULE] " + AnsiWhite + target + " -> " + tierName + AnsiReset)
		_, _ = os.Stdout.WriteString(color + "[SUCCESS] Target " + AnsiWhite + target + color + " successfully set to " + tierName + "\n" + AnsiReset)
	}
}

func RenderCLIMenu(engine *Engine) {
	// Kompatibilitas CLI Shell
}
