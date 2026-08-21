package main

import (
	"net"
	"os"

	"github.com/rs/zerolog/log"
	"golang.org/x/sys/unix"
)

func FetchRulesFromGithub() {
	home := os.Getenv("HOME")
	if home == "" {
		home = "/data/data/com.termux/files/home"
	}
	basePath := home + "/.rules/"

	files := map[string]byte{
		basePath + "adguarddns.txt":         CodeBlock,
		basePath + "easyprivacy.txt":        CodeBlock,
		basePath + "easylist.txt":           CodeBlock,
		basePath + "blocklist-ads.txt":      CodeBlock,
		basePath + "ip-telegram.txt":        CodeVVIP,
		basePath + "ip-google.txt":          CodeVVIP,
	}

	log.Info().Msg("Memulai pemetaan rules ke Fastcache & Radix Trie...")

	for path, tier := range files {
		f, err := os.Open(path)
		if err != nil {
			if LogErrors {
				log.Warn().Str("file", path).Msg("File rules tidak ditemukan/gagal dibuka")
			}
			continue
		}

		stat, err := f.Stat()
		if err != nil || stat.Size() == 0 {
			f.Close()
			continue
		}

		data, err := unix.Mmap(int(f.Fd()), 0, int(stat.Size()), unix.PROT_READ, unix.MAP_SHARED)
		f.Close()
		if err != nil {
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

		unix.Munmap(data)
		log.Info().Str("file", path).Msg("Rules sukses diparsing")
	}

	log.Info().Uint64("total_rules", TotalLoadedRules.Load()).Msg("🔥 SEMUA RULES AKTIF")
}

func addCIDRToTrie(ip net.IP, mask net.IPMask, tier byte) {
	isV4 := ip.To4() != nil
	root := IPv6Trie
	rawIP := ip.To16()
	if isV4 {
		root = IPv4Trie
		rawIP = ip.To4()
	}

	node := root
	ones, _ := mask.Size()

	for i := 0; i < ones; i++ {
		byteIndex := i / 8
		bitIndex := 7 - (i % 8)
		bit := (rawIP[byteIndex] >> bitIndex) & 1

		if node.Children[bit] == nil {
			node.Children[bit] = &TrieNode{}
		}
		node = node.Children[bit]
	}
	node.HasRule = true
	node.Tier = tier
	TotalLoadedRules.Add(1)
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
			ip, ipnet, err := net.ParseCIDR(strLine)
			if err == nil {
				addCIDRToTrie(ip, ipnet.Mask, tier)
				return
			}
		} else {
			ip := net.ParseIP(strLine)
			if ip != nil {
				mask := net.CIDRMask(128, 128)
				if ip.To4() != nil {
					mask = net.CIDRMask(32, 32)
				}
				addCIDRToTrie(ip, mask, tier)
				return
			}
		}
	}

	DBEngine.Set(line, []byte{tier})
	TotalLoadedRules.Add(1)
}
