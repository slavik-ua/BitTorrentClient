#!/bin/bash
set -e
SEED_DIR="seed/data"
mkdir -p "$SEED_DIR"

echo "Generating 20MB file..."
dd if=/dev/urandom of="$SEED_DIR/file20.bin" bs=1M count=20 status=progress

echo "Generating 40MB file..."
dd if=/dev/urandom of="$SEED_DIR/file40.bin" bs=1M count=40 status=progress

echo "Generating small text file..."
echo "Hello torrent local network - $(date)" > "$SEED_DIR/readme.txt"
for i in {1..500}; do echo "Line $i - $(head -c 100 /dev/urandom | base64)" >> "$SEED_DIR/readme.txt"; done

echo ""
echo "Seed files created:"
ls -lh "$SEED_DIR/"
du -sh "$SEED_DIR"
