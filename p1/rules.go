package main

import (
	"bufio"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/rs/zerolog"
)

type TrieNode struct {
	Children [2]*TrieNode
	Tier     byte
	HasRule  bool
}

type BinaryRadixTrie struct {
	root *TrieNode
}

func NewBinaryRadixTrie() *BinaryRadixTrie {
	return &BinaryRadixTrie{root: &TrieNode{}}
}

func (t *BinaryRadixTrie) Insert(ipNet *net.IPNet, tier byte) {
	ones, _ := ipNet.Mask.Size()
	if ones == 0 {
		return
	}

	curr := t.root
	ip := ipNet.IP
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}

	for i := 0; i < ones; i++ {
		byteIdx := i / 8
		bitIdx := 7 - (i % 8)
		bit := (ip[byteIdx] >> bitIdx) & 1

		if curr.Children[bit] == nil {
			curr.Children[bit] = &TrieNode{}
		}
		curr = curr.Children[bit]
	}
	curr.Tier = tier
	curr.HasRule = true
}

func (t *BinaryRadixTrie) Match(ip net.IP) (byte, bool) {
	curr := t.root
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}

	var lastTier byte
	var matched bool

	if curr.HasRule {
		lastTier = curr.Tier
		matched = true
	}

	totalBits := len(ip) * 8
	for i := 0; i < totalBits; i++ {
		byteIdx := i / 8
		bitIdx := 7 - (i % 8)
		bit := (ip[byteIdx] >> bitIdx) & 1

		if curr.Children[bit] == nil {
			break
		}
		curr = curr.Children[bit]
		if curr.HasRule {
			lastTier = curr.Tier
			matched = true
		}
	}
	return lastTier, matched
}

var restrictedTLDs = map[string]bool{
	"com": true, "net": true, "org": true, "id": true, "co.id": true,
	"or.id": true, "go.id": true, "ac.id": true, "biz.id": true,
	"web.id": true, "my.id": true, "co": true, "io": true, "xyz": true,
	"info": true, "biz": true, "us": true, "uk": true, "me": true,
	"top": true, "site": true, "online": true, "edu": true, "gov": true,
}

type RuleManager struct {
	v4Trie    *BinaryRadixTrie
	v6Trie    *BinaryRadixTrie
	domainDB  *fastcache.Cache
	telemetry *TelemetryTracker
	zlog      zerolog.Logger
}

func NewRuleManager(db *fastcache.Cache, telemetry *TelemetryTracker, logger zerolog.Logger) *RuleManager {
	rm := &RuleManager{
		v4Trie:    NewBinaryRadixTrie(),
		v6Trie:    NewBinaryRadixTrie(),
		domainDB:  db,
		telemetry: telemetry,
		zlog:      logger,
	}

	rm.injectBuiltinVVIPRules()
	return rm
}

func (rm *RuleManager) injectBuiltinVVIPRules() {
	telegramCIDRs := []string{
		"149.154.160.0/20", "149.154.164.0/22", "149.154.168.0/22",
		"149.154.172.0/22", "91.108.4.0/22", "91.108.8.0/22",
		"91.108.12.0/22", "91.108.16.0/22", "91.108.20.0/22",
		"91.108.56.0/22", "95.161.64.0/20",
	}
	for _, cidr := range telegramCIDRs {
		_ = rm.AddIPRule(cidr, CodeVVIP)
	}
}

func IsAbsoluteWhitelisted(clean string) bool {
	whitelistKeywords := []string{
		"google.com", "google.co.id", "googleapis.com", "gstatic.com",
		"googleusercontent.com", "googlevideo.com", "gvt1.com", "1e100.net",
		"recaptcha.net", "googletagmanager.com", "google-analytics.com",
		"myrepublic.co.id", "myrepublic.net", "myrepublic.com",
		"speedtest.net", "ookla.com", "speedtestcustom.com", "ooklaserver.net",
		"measurementlab.net", "measurement-lab.org", "m-lab.org", "mlab.org", // Google Speedtest M-Lab Engine
		"fast.com", "nflxvideo.net", "netflix.com",
		"cloudflare.com", "speed.cloudflare.com", "cloudflare.net",
		"t.me", "telegram.org", "telegram.me", "telesco.pe", "tx.me",
		"cdn-telegram.org", "stel.com", "tdesktop.com",
	}

	for _, kw := range whitelistKeywords {
		if clean == kw || strings.HasSuffix(clean, "."+kw) {
			return true
		}
	}
	return false
}

func (rm *RuleManager) AddIPRule(cidrStr string, tier byte) error {
	var ipNet *net.IPNet
	if !strings.Contains(cidrStr, "/") {
		ip := net.ParseIP(cidrStr)
		if ip == nil {
			return net.InvalidAddrError("invalid IP")
		}
		if v4 := ip.To4(); v4 != nil {
			ipNet = &net.IPNet{IP: v4, Mask: net.CIDRMask(32, 32)}
		} else {
			ipNet = &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}
		}
	} else {
		var err error
		_, ipNet, err = net.ParseCIDR(cidrStr)
		if err != nil {
			return err
		}
	}

	if ipNet.IP.To4() != nil {
		rm.v4Trie.Insert(ipNet, tier)
	} else {
		rm.v6Trie.Insert(ipNet, tier)
	}
	rm.telemetry.TotalLoadedRules.Add(1)
	return nil
}

func (rm *RuleManager) AddDomainRule(domain string, tier byte) {
	cleanDomain := strings.ToLower(strings.TrimSpace(domain))
	if restrictedTLDs[cleanDomain] || IsAbsoluteWhitelisted(cleanDomain) {
		return
	}

	key := []byte(cleanDomain)
	rm.domainDB.Set(key, []byte{tier})
	rm.telemetry.TotalLoadedRules.Add(1)
}

// ForceSetDomainRule mengubah rule domain secara dinamis via CLI command
func (rm *RuleManager) ForceSetDomainRule(domain string, tier byte) {
	cleanDomain := strings.ToLower(strings.TrimSpace(domain))
	key := []byte(cleanDomain)
	rm.domainDB.Set(key, []byte{tier})
	rm.telemetry.TotalLoadedRules.Add(1)
}

// ForceSetIPRule mengubah rule IP secara dinamis via CLI command
func (rm *RuleManager) ForceSetIPRule(ipStr string, tier byte) error {
	return rm.AddIPRule(ipStr, tier)
}

func (rm *RuleManager) EvaluateIP(ip net.IP) byte {
	if v4 := ip.To4(); v4 != nil {
		if tier, ok := rm.v4Trie.Match(v4); ok {
			return tier
		}
	} else {
		if tier, ok := rm.v6Trie.Match(ip); ok {
			return tier
		}
	}
	return CodeReguler
}

func (rm *RuleManager) EvaluateDomain(domain string) byte {
	clean := strings.ToLower(strings.TrimSpace(domain))

	// 1. Cek Exact Override di DB terlebih dahulu
	buf := []byte(clean)
	var val [1]byte
	res := rm.domainDB.Get(val[:0], buf)
	if len(res) > 0 {
		return res[0]
	}

	// 2. Cek Whitelist Bawaan
	if IsAbsoluteWhitelisted(clean) {
		return CodeVVIP
	}

	// 3. Backwards Hierarchical Scan
	for i := 0; i < len(clean); i++ {
		if clean[i] == '.' {
			sub := clean[i+1:]
			if restrictedTLDs[sub] || !strings.Contains(sub, ".") {
				break
			}
			res = rm.domainDB.Get(val[:0], []byte(sub))
			if len(res) > 0 {
				return res[0]
			}
		}
	}
	return CodeReguler
}

func CleanRuleLine(raw string) (clean string, isIP bool, skip bool) {
	line := strings.TrimSpace(raw)
	if len(line) == 0 {
		return "", false, true
	}

	if line[0] == '#' || line[0] == '!' || line[0] == ';' || line[0] == '[' {
		return "", false, true
	}
	if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "@@") || strings.Contains(line, "##") || strings.Contains(line, "#@#") {
		return "", false, true
	}

	if idx := strings.IndexByte(line, '#'); idx != -1 {
		line = strings.TrimSpace(line[:idx])
		if len(line) == 0 {
			return "", false, true
		}
	}

	if strings.Contains(line, ",") {
		parts := strings.Split(line, ",")
		if len(parts) >= 2 {
			tag := strings.ToUpper(strings.TrimSpace(parts[0]))
			val := strings.TrimSpace(parts[1])
			if tag == "DOMAIN" || tag == "DOMAIN-SUFFIX" || tag == "DOMAIN-KEYWORD" || tag == "HOST" || tag == "HOST-SUFFIX" {
				line = val
			} else if tag == "IP-CIDR" || tag == "IP-CIDR6" || tag == "IP-SUFFIX" {
				line = val
			} else {
				return "", false, true
			}
		}
	}

	if strings.HasPrefix(line, "domain:") || strings.HasPrefix(line, "full:") || strings.HasPrefix(line, "keyword:") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			line = strings.TrimSpace(parts[1])
		}
	}

	tokens := strings.Fields(line)
	if len(tokens) >= 2 {
		first := tokens[0]
		if first == "0.0.0.0" || first == "127.0.0.1" || first == "::1" || first == "::" {
			line = tokens[1]
		}
	}

	if strings.HasPrefix(line, "||") {
		line = strings.TrimPrefix(line, "||")
	}
	if idx := strings.IndexByte(line, '$'); idx != -1 {
		line = line[:idx]
	}
	if idx := strings.IndexByte(line, '^'); idx != -1 {
		line = line[:idx]
	}

	line = strings.Trim(line, " \t\r\n/.")
	if len(line) == 0 {
		return "", false, true
	}

	if line == "0.0.0.0" || line == "127.0.0.1" || line == "localhost" || line == "broadcasthost" || line == "local" || line == "0.0.0.0/0" || line == "::/0" {
		return "", false, true
	}

	if strings.Contains(line, "/") {
		if strings.HasPrefix(line, "0.0.0.0/") || strings.HasPrefix(line, "::/") {
			return "", false, true
		}
		if _, _, err := net.ParseCIDR(line); err == nil {
			return line, true, false
		}
	}

	if ip := net.ParseIP(line); ip != nil {
		if ip.IsUnspecified() || ip.IsLoopback() {
			return "", false, true
		}
		return line, true, false
	}

	clean = strings.ToLower(line)
	if !strings.Contains(clean, ".") || restrictedTLDs[clean] || strings.Contains(clean, "/") || strings.Contains(clean, ":") {
		return "", false, true
	}

	return clean, false, false
}

func (rm *RuleManager) LoadRuleFile(filePath string, tier byte) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	count := 0
	for scanner.Scan() {
		clean, isIP, skip := CleanRuleLine(scanner.Text())
		if skip {
			continue
		}

		if isIP {
			if err := rm.AddIPRule(clean, tier); err == nil {
				count++
			}
		} else {
			rm.AddDomainRule(clean, tier)
			count++
		}
	}
	return count, scanner.Err()
}

func (rm *RuleManager) LoadRulesFromDirectory(dirPath string) {
	if strings.HasPrefix(dirPath, "~/") || dirPath == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			dirPath = filepath.Join(home, dirPath[1:])
		}
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		rm.zlog.Warn().Str("dir", dirPath).Msg("rules directory not found")
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := strings.ToLower(entry.Name())
		fullPath := filepath.Join(dirPath, entry.Name())

		var tier byte = CodeReguler

		if strings.Contains(name, "telegram") || strings.Contains(name, "google") || strings.Contains(name, "vvip") {
			tier = CodeVVIP
		} else if strings.Contains(name, "adaway") ||
			strings.Contains(name, "adguard") ||
			strings.Contains(name, "ads") ||
			strings.Contains(name, "malware") ||
			strings.Contains(name, "tracking") ||
			strings.Contains(name, "easylist") ||
			strings.Contains(name, "easyprivacy") ||
			strings.Contains(name, "hagezi") ||
			strings.Contains(name, "reject") ||
			strings.Contains(name, "stevenblack") ||
			strings.Contains(name, "block") {
			tier = CodeBlock
		}

		loaded, err := rm.LoadRuleFile(fullPath, tier)
		if err != nil {
			rm.zlog.Error().Err(err).Str("file", entry.Name()).Msg("failed loading rule file")
		} else {
			rm.zlog.Info().
				Str("file", entry.Name()).
				Int("count", loaded).
				Str("tier", TierToString(tier)).
				Msg("rule file indexed")
		}
	}

	rm.injectBuiltinVVIPRules()
}
