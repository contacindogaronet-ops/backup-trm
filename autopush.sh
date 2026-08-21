#!/data/data/com.termux/files/usr/bin/bash

cd "$HOME" || exit 1

# Cek apakah ada perubahan file (untracked/modified/deleted)
if [ -n "$(git status --porcelain)" ]; then
    TIMESTAMP=$(date "+%Y-%m-%d %H:%M:%S")
    git add -A
    git commit -m "chore(backup): auto-sync $TIMESTAMP"
    git push origin main --quiet
fi
