#!/usr/bin/env python3
# scenes-contact-sheet.py — one PNG per scene with a 2x2 grid of frames
# (t = 0, 3, 8, 15 s) at 100x30, plus the same sheet at 24x10, 40x14 and
# 200x50, and splash dumps at t = 0.3, 1.2, 2.8 s at 100x30 and 200x50.
# Tile order in every grid: top-left = t0, top-right = t3, t8, t15.
# Written under a persistent directory (default: mktemp -d), never into
# the repo. Prints every path it writes.

import os
import re
import subprocess
import sys

OUT = sys.argv[1] if len(sys.argv) > 1 else None
SCENES = ["night", "bonsai", "ocean", "rain", "koi", "snow", "campfire", "mountains", "river", "meteor"]
SIZES = ["24x10", "40x14", "100x30", "200x50"]
TIMES = [0, 3, 8, 15]
SCALE = {"24x10": 10, "40x14": 6, "100x30": 3, "200x50": 2}
SPLASH_TIMES = [0.3, 1.2, 2.8]
SPLASH_SIZES = ["100x30", "200x50"]

repo_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
bin_path = os.path.join(repo_root, "bin", "mtm")

if not os.path.exists(bin_path):
    # build if the binary is missing (sandboxed, shared Go caches)
    subprocess.run(["go", "build", "-o", bin_path, "./cmd/mtm"], cwd=repo_root, check=True)

out_dir = OUT or subprocess.check_output(["mktemp", "-d", "/tmp/mtm-contact-sheet-XXXXXX"], text=True).strip()
os.makedirs(out_dir, exist_ok=True)

# Sandbox for the renders: temp HOME + XDG roots. The night scene is the
# art reference, so the sandbox config pins it (the contact sheets for the
# other scenes use --scene anyway).
env = dict(os.environ)
sandbox_dir = subprocess.check_output(["mktemp", "-d", "/tmp/mtm-cs-sandbox-XXXXXX"], text=True).strip()
home = os.path.join(sandbox_dir, "home")
env.update({
    "MTM_SANDBOX": "1",
    "HOME": home,
    "XDG_DATA_HOME": os.path.join(home, ".local/share"),
    "XDG_CONFIG_HOME": os.path.join(home, ".config"),
    "XDG_CACHE_HOME": os.path.join(home, ".cache"),
    "XDG_STATE_HOME": os.path.join(home, ".local/state"),
    "OMARCHY_STATE_HOME": os.path.join(home, ".local/state/omarchy"),
})
os.makedirs(os.path.join(home, ".config/mtm"), exist_ok=True)
with open(os.path.join(home, ".config/mtm/config.yaml"), "w") as f:
    f.write("scene: night\n")


def dump(scene, size, t, progress=0.45, ppm=None):
    out = [bin_path, "tui", "--dump", "--ppm", ppm or "/tmp/_dump.ppm",
           "--ppm-scale", str(SCALE.get(size, 3)), "--scene", scene,
           "--t", f"{t}s", "--progress", str(progress), "--size", size]
    r = subprocess.run(out, env=env, capture_output=True, text=True, check=True)
    if ppm is None:
        raise RuntimeError("--scene dump requires --ppm")


def dump_splash(size, t, ppm):
    out = [bin_path, "tui", "--dump", "--ppm", ppm,
           "--ppm-scale", str(SCALE.get(size, 3)),
           "--splash-t", str(t), "--size", size]
    subprocess.run(out, env=env, capture_output=True, text=True, check=True)


def read_ppm(path):
    with open(path, "rb") as f:
        magic = f.readline().strip()
        if magic != b"P6":
            raise ValueError(f"not a P6 ppm: {path}")
        dims = f.readline()
        while dims.startswith(b"#"):
            dims = f.readline()
        w, h = map(int, dims.split())
        maxval = int(f.readline())
        data = f.read(w * h * 3)
        return w, h, data


def write_ppm(path, w, h, data):
    with open(path, "wb") as f:
        f.write(f"P6\n{w} {h}\n255\n".encode())
        f.write(data)


def compose_grid(tile_paths, out_path, gap=6, bg=(8, 10, 12)):
    tiles = [read_ppm(p) for p in tile_paths]
    tw = max(t[0] for t in tiles)
    th = max(t[1] for t in tiles)
    w, h = 2 * tw + gap, 2 * th + gap
    canvas = bytearray(bg * (w * h))

    def blit(dst_x, dst_y, tw, th, data, sw, rows_stride):
        for row in range(th):
            src = row * sw * 3
            for col in range(tw):
                si = src + col * 3
                di = ((dst_y + row) * w + dst_x + col) * 3
                canvas[di:di + 3] = data[si:si + 3]

    positions = [(0, 0), (tw + gap, 0), (0, th + gap), (tw + gap, th + gap)]
    for tile, (dx, dy) in zip(tiles, positions):
        sw, sh, data = tile
        if sw != tw or sh != th:
            # center smaller tiles on their grid cell
            dx += (tw - sw) // 2
            dy += (th - sh) // 2
        blit(dx, dy, sw, sh, data, sw, sw)
    write_ppm(out_path, w, h, bytes(canvas))


def png_from_ppm(ppm_path, png_path):
    with open(png_path, "wb") as f:
        subprocess.run(["pnmtopng", ppm_path], stdout=f, check=True)


sheets = []
for scene in SCENES:
    for size in SIZES:
        tile_ppm = []
        for t in TIMES:
            p = os.path.join(out_dir, f"_tile-{scene}-{size}-t{t}.ppm")
            dump(scene, size, t, ppm=p)
            tile_ppm.append(p)
        sheet_ppm = os.path.join(out_dir, f"sheet-{scene}-{size}.ppm")
        sheet_png = sheet_ppm[:-4] + ".png"
        compose_grid(tile_ppm, sheet_ppm, gap=6)
        png_from_ppm(sheet_ppm, sheet_png)
        for p in tile_ppm:
            os.unlink(p)
        os.unlink(sheet_ppm)
        sheets.append(sheet_png)

for t in SPLASH_TIMES:
    for size in SPLASH_SIZES:
        ppm = os.path.join(out_dir, f"splash-t{str(t).replace('.', 'p')}-{size}.ppm")
        png = ppm[:-4] + ".png"
        dump_splash(size, t, ppm)
        png_from_ppm(ppm, png)
        os.unlink(ppm)
        sheets.append(png)

# clean up the render sandbox
subprocess.run(["rm", "-rf", sandbox_dir], check=False)

print(f"contact sheets written to {out_dir}")
print("tile order in every grid: top-left t0, top-right t3, bottom-left t8, bottom-right t15")
for s in sheets:
    print(s)
