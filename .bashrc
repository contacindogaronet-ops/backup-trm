# 1. Pasang Wake-Lock
# Ini nyuruh CPU buat nolak mode tidur (Deep Sleep) selama Termux jalan.
termux-wake-lock

# 2. Paksa resolve DNS lokal pakai Cloudflare
# Ganti nameserver bawaan OS dengan CF & Google
echo "nameserver 1.1.1.1" > $PREFIX/etc/resolv.conf
echo "nameserver 8.8.8.8" >> $PREFIX/etc/resolv.conf

# 3. Tuning Buffer Socket via Environment Variable
# (Membantu aplikasi Golang/lainnya yang jalan di Termux buat alokasi socket buffer lebih lega)
export SO_RCVBUF=4194304
export SO_SNDBUF=4194304


# ==========================================
# GOLANG RAPID DEVELOPMENT TOOLS
# ==========================================

# 1. Wrapper 'go build' (Native Overwrite & LDFLAGS Optimization)
build() {
    if [[ -z "$1" ]]; then
        echo -e "\e[31m[FATAL]\e[0m Output name missing. Usage: build <output-bin> [target]"
        return 1
    fi

    local bin_name="$1"
    local target="${2:-.}"

    echo -e "\e[34m[INFO]\e[0m Compiling with LDFLAGS (-s -w)..."
    
    # Eksekusi build. Go otomatis melakukan replace/overwrite file binary lama.
    go build -ldflags="-s -w" -o "$bin_name" "$target"

    if [[ $? -eq 0 ]]; then
        echo -e "\e[32m[SUCCESS]\e[0m Executable ready -> ./${bin_name}"
    else
        echo -e "\e[31m[ERROR]\e[0m Compilation failed. Check your syntax."
        return 1
    fi
}

# 2. Shortcut Eksekusi 'go run' via Ctrl+R
bind '"\C-r": "clear; echo -e \"\\e[33m[RUNNING]\\e[0m go run .\"; go run .\n"'

# ==========================================
