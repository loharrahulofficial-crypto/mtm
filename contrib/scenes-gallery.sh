#!/bin/sh
# scenes-gallery.sh — one PPM plus one PNG per scene x size into ${2:-temp}.
# Usage: scenes-gallery.sh <mtm-binary> [out-dir]
# Sizes cover the shapes mtm actually ships at: sidebar windows, the Home
# corner, a small Focus pane, and full-screen splashes.
set -eu
BIN=$1
OUT=${2:-$(mktemp -d /tmp/mtm-scenes-XXXXXX)}
. "$(dirname "$0")/sandbox-env.sh"
mkdir -p "$OUT"
sizes="24x10 30x16 40x14 64x16 100x30 200x50"
scenes="night bonsai ocean rain koi snow campfire mountains river meteor"
for scene in $scenes; do
  for size in $sizes; do
    ppm="$OUT/${scene}-${size}.ppm"
    png="$OUT/${scene}-${size}.png"
    MTM_SANDBOX=1 "$BIN" tui --dump --scene "$scene" --size "$size" --t 3s --progress 0.45 --ppm "$ppm" >/dev/null
    pnmtopng "$ppm" > "$png"
    echo "$ppm"
    echo "$png"
  done
done
echo "gallery written to $OUT"
