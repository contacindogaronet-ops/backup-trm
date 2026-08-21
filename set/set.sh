#!/bin/bash

echo "============================================="
echo "   ADB BLOATWARE & TELEMETRY REMOVER (SAFE)  "
echo "============================================="

# Koneksi lokal / port input
read -p "Masukkan port ADB (Contoh: 5555) atau kosongkan jika sudah connect: " port_input

if [ -n "$port_input" ]; then
    echo "Menghubungkan ke 127.0.0.1:$port_input..."
    adb connect 127.0.0.1:$port_input
else
    echo "Menggunakan koneksi ADB yang aktif..."
fi

adb devices
echo "Memulai pembersihan bloatware aman..."

# --- 1. XIAOMI / MIUI BLOATWARE & TRACKER ---
XIAOMI_BLOAT=(
    com.miui.analytics
    com.miui.msa.global
    com.xiaomi.mipicks
    com.miui.yellowpage
    com.miui.hybrid
    com.miui.cloudservice
    com.miui.cloudbackup
    com.miui.weather2
    com.xiaomi.payment
    com.xiaomi.scanner
    com.miui.notes
    com.xiaomi.market
    com.miui.voiceassist
    com.miui.cleaner
    com.miui.backup
    com.miui.qr
    com.xiaomi.barrage
    com.miui.phrase
    com.miui.thirdappassistant
)

for pkg in "${XIAOMI_BLOAT[@]}"; do
    adb shell pm uninstall -k --user 0 "$pkg" 2>/dev/null && echo "[HAPUS] $pkg"
done

# --- 2. GOOGLE BLOATWARE (KECUALI GOOGLE UTAMA / GEMINI) ---
GOOGLE_BLOAT=(
    com.google.android.youtube
    com.google.android.apps.youtube.music
    com.google.android.apps.photos
    com.google.android.gm
    com.google.android.videos
    com.google.android.apps.tachyon
    com.google.android.printservice
    com.google.android.tts
    com.google.android.feedback
    com.google.android.printservice.recommendation
    com.google.android.apps.docs
)

for pkg in "${GOOGLE_BLOAT[@]}"; do
    adb shell pm uninstall -k --user 0 "$pkg" 2>/dev/null && echo "[HAPUS] $pkg"
done


echo "============================================="
echo "   SELESAI! GOOGLE & GEMINI AMAN.            "
echo "============================================="
