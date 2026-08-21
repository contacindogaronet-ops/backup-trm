#!/bin/bash

# Buat folder khusus rules kalau belum ada
mkdir -p rules

echo "🔥 Memulai download jutaan rules ke folder lokal..."

# 🔴 DOMAIN BLOCK/ADS/TRACKER
curl -sLo rules/adguarddns.txt "https://raw.githubusercontent.com/justdomains/blocklists/master/lists/adguarddns-justdomains.txt"
curl -sLo rules/easyprivacy.txt "https://raw.githubusercontent.com/justdomains/blocklists/master/lists/easyprivacy-justdomains.txt"
curl -sLo rules/easylist.txt "https://raw.githubusercontent.com/justdomains/blocklists/master/lists/easylist-justdomains.txt"
curl -sLo rules/adaway.txt "https://raw.githubusercontent.com/AdAway/adaway.github.io/master/hosts.txt"
curl -sLo rules/reject-list.txt "https://raw.githubusercontent.com/Loyalsoldier/v2ray-rules-dat/release/reject-list.txt"
curl -sLo rules/hagezi-pro.txt "https://raw.githubusercontent.com/hagezi/dns-blocklists/main/domains/pro.plus.txt"
curl -sLo rules/blocklist-ads.txt "https://raw.githubusercontent.com/blocklistproject/Lists/master/ads.txt"
curl -sLo rules/blocklist-tracking.txt "https://raw.githubusercontent.com/blocklistproject/Lists/master/tracking.txt"
curl -sLo rules/blocklist-malware.txt "https://raw.githubusercontent.com/blocklistproject/Lists/master/malware.txt"
curl -sLo rules/v2fly-ads.txt "https://raw.githubusercontent.com/v2fly/domain-list-community/master/data/category-ads-all"
curl -sLo rules/stevenblack.txt "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts"

# 🟢 WHITELIST & TIERING DOMAIN (VVIP/VIP)
curl -sLo rules/v2fly-google.txt "https://raw.githubusercontent.com/v2fly/domain-list-community/master/data/google"
curl -sLo rules/v2fly-telegram.txt "https://raw.githubusercontent.com/v2fly/domain-list-community/master/data/telegram"

# 🔵 IP CIDR LISTS (GEOIP)
curl -sLo rules/ip-telegram.txt "https://raw.githubusercontent.com/Loyalsoldier/geoip/release/text/telegram.txt"
curl -sLo rules/ip-google.txt "https://raw.githubusercontent.com/Loyalsoldier/geoip/release/text/google.txt"

echo "✅ Download selesai! Semua raw text sudah tersimpan di folder 'rules/'."
