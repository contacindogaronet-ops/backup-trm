package main

import (
	"net"
	"os"

	"github.com/rs/zerolog/log"
	"golang.org/x/sys/unix"
)

// ==========================================
// ZERO-ALLOC LOCAL FILE PARSER (MMAP)
// ==========================================
func FetchRulesFromGithub() {
		home := os.Getenv("HOME")
	if home == "" {
		home = "/data/data/com.termux/files/home" // Fallback Termux
	}
	basePath := home + "/.rules/"

	files := map[string]byte{
		// 🔴 DOMAIN BLOCK/ADS/TRACKER
		basePath + "adguarddns.txt":         CodeBlock,
		basePath + "easyprivacy.txt":        CodeBlock,
		basePath + "easylist.txt":           CodeBlock,
		basePath + "adaway.txt":             CodeBlock,
		basePath + "reject-list.txt":        CodeBlock,
		basePath + "hagezi-pro.txt":         CodeBlock,
		basePath + "blocklist-ads.txt":      CodeBlock,
		basePath + "blocklist-tracking.txt": CodeBlock,
		basePath + "blocklist-malware.txt":  CodeBlock,
		basePath + "v2fly-ads.txt":          CodeBlock,
		basePath + "stevenblack.txt":        CodeBlock,

		// 🟢 WHITELIST & TIERING DOMAIN (VVIP)
		basePath + "v2fly-google.txt":   CodeVVIP,
		basePath + "v2fly-telegram.txt": CodeVVIP, 

		// 🔵 IP CIDR LISTS (GEOIP)
		basePath + "ip-telegram.txt": CodeVVIP, 
		basePath + "ip-google.txt":   CodeVVIP,
	}

	log.Info().Msg("Memulai pemetaan jutaan Rules via MMAP (Zero-Copy Heap)...")

	for path, tier := range files {
		f, err := os.Open(path)
		if err != nil {
			log.Error().Err(err).Str("file", path).Msg("Gagal buka file rules")
			continue
		}

		stat, err := f.Stat()
		if err != nil || stat.Size() == 0 {
			f.Close()
			continue
		}

		// Injeksi Mmap: Map file langsung ke OS Page Cache
		data, err := unix.Mmap(int(f.Fd()), 0, int(stat.Size()), unix.PROT_READ, unix.MAP_SHARED)
		f.Close() // File descriptor bisa ditutup setelah di-map
		if err != nil {
			log.Error().Err(err).Str("file", path).Msg("Gagal mmap file")
			continue
		}

		start := 0
		for i := 0; i < len(data); i++ {
			if data[i] == '\n' {
				injectToDB(data[start:i], tier)
				start = i + 1
			}
		}
		if start < len(data) {
			injectToDB(data[start:], tier)
		}

		// Lepas memori dari OS setelah selesai parsing
		unix.Munmap(data)

		log.Info().Str("file", path).Msg("Rules lokal sukses diparsing via Mmap")
	}

	// 🔴 INJEKSI MANUAL KHUSUS (HARD BLOCK & WHITELIST)
	manualRules := map[string]byte{
		// TikTok / ByteDance Ads & Telemetry
		"pangle.io":     CodeBlock,
		"pglstatp.com":  CodeBlock,
		"tiktokcdn.com": CodeBlock,
		"ibyteimg.com":  CodeBlock,
		
		// Novel Ads
		"fizzopic.org":    CodeBlock,
		"byteoversea.com": CodeBlock,
		
		// Shopee Tracker
		"mmp.shopee.co.id": CodeBlock,
		
		// Target Statis Uji Coba
		"cp.cloudflare.com": CodeVIP,

		// 🟢 WHITELIST APLIKASI NOVEL
		"tmtreader.com": CodeReguler,
	}

	for domain, tier := range manualRules {
		DBEngine.Set([]byte(domain), []byte{tier})
		TotalLoadedRules.Add(1)
	}
	
	for rule, tier := range manualRules {
		// PERBAIKAN: Lempar ke zero-alloc parser biar IP & Domain terpisah dengan benar
		injectToDB([]byte(rule), tier)
	}
	
	log.Info().
		Uint64("total_rules", TotalLoadedRules.Load()).
		Int("total_ip_cidr", len(IPRules)).
		Msg("🔥 SEMUA RULES LOKAL BERHASIL DIINJEK (MMAP)")
}

func injectToDB(line []byte, tier byte) {
	start := 0
	for start < len(line) && (line[start] == ' ' || line[start] == '\t' || line[start] == '\r') {
		start++
	}
	line = line[start:]

	if len(line) == 0 || line[0] == '#' {
		return
	}

	end := len(line)
	for i := 0; i < len(line); i++ {
		if line[i] == '#' {
			end = i
			break
		}
	}
	for end > 0 && (line[end-1] == ' ' || line[end-1] == '\t' || line[end-1] == '\r') {
		end--
	}
	line = line[:end]
	if len(line) == 0 {
		return
	}

	if len(line) > 8 && line[0] == '0' && line[1] == '.' && line[2] == '0' && line[3] == '.' && line[4] == '0' && line[5] == '.' && line[6] == '0' && line[7] == ' ' {
		line = line[8:]
	} else if len(line) > 10 && line[0] == '1' && line[1] == '2' && line[2] == '7' && line[3] == '.' && line[4] == '0' && line[5] == '.' && line[6] == '0' && line[7] == '.' && line[8] == '1' && line[9] == ' ' {
		line = line[10:]
	}

	isIP := true
	hasSlash := false
	for i := 0; i < len(line); i++ {
		b := line[i]
		if b == '/' {
			hasSlash = true
			continue
		}
		if (b >= '0' && b <= '9') || b == ':' || b == '.' {
			continue
		}
		isIP = false
		break
	}

	if isIP {
		strLine := string(line)
		if hasSlash {
			_, ipnet, err := net.ParseCIDR(strLine)
			if err == nil {
				IPRules = append(IPRules, IPRule{Net: ipnet, Tier: tier})
				TotalLoadedRules.Add(1)
				return
			}
		} else {
			ip := net.ParseIP(strLine)
			if ip != nil {
				var mask net.IPMask
				if ip.To4() != nil {
					mask = net.CIDRMask(32, 32)
				} else {
					mask = net.CIDRMask(128, 128)
				}
				ipnet := &net.IPNet{IP: ip, Mask: mask}
				IPRules = append(IPRules, IPRule{Net: ipnet, Tier: tier})
				TotalLoadedRules.Add(1)
				return
			}
		}
	}

	DBEngine.Set(line, []byte{tier})
	TotalLoadedRules.Add(1)
}
