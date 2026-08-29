#!/bin/bash
echo "🔥 [JARGO] Memulai Protokol Bumi Hangus..."

# 1. Amputasi Fisik
echo "[-] Menghapus folder core dan libs..."
rm -rf core/
rm -rf android/app/libs/
rm -rf android/app/src/main/assets/core_engine

# 2. Amputasi Memori Git
echo "[-] Mencabut memori Git..."
git rm -r --cached core/ 2>/dev/null
git rm -r --cached android/app/libs/ 2>/dev/null

# 3. Kunci Realitas Baru
echo "[+] Merekam arsitektur baru..."
git add .
git commit -m "chore(reset): eksekusi nuke script untuk transisi ke daemon manager"

# 4. Tembakan Absolut
echo "🚀 Menembakkan ke infrastruktur Cloud..."
git push -f origin main

echo "✅ [JARGO] Eksekusi Selesai."
