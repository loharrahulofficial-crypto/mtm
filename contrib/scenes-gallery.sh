#!/bin/sh
# scenes-gallery.sh — write one PPM per scene x time x size into $2.
# Usage: scenes-gallery.sh <mtm-binary> <out-dir>
set -eu
BIN=$1
OUT=$2
. "$(dirname "$0")/sandbox-env.sh"
mkdir -p "$OUT"
for scene in night bonsai ocean rain koi snow campfire; do
  for t in 0 3 8; do
    MTM_SANDBOX=1 "$BIN" tui --dump --scene "$scene" --size 100x30 --t "${t}s" --progress 0.45 --ppm "$OUT/${scene}-t${t}.ppm" >/dev/null
  done
done
echo "wrote $OUT"
