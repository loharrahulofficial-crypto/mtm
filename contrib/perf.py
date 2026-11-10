#!/usr/bin/env python3
"""perf.py — measure mtm's CPU and paint cost under a pty, sandboxed.

Usage: perf.py [--size WxH] [--duration SECONDS] [--settle SECONDS] [--out DIR]
               [--binary PATH] [scenario ...]

Scenarios (120x40 by default):
  home-ambient-off   Home, ambient off, idle (the ≤ 2% target)
  home-ambient-12fps Home with the sidebar window on, ui.ambient_fps: 12 (≤ 20%)
  focus-24fps        Focus mode with a running Pomodoro, fps: 24      (≤ 30%)
  idle-ambient-5m    Ambient on, untouched; --duration defaults to 300 (≤ 5%)

Metrics printed per scenario: user/sys CPU seconds (from wait4), user+sys as
% of one core over the wall time, paints/s (alt-screen flushes counted as
ESC[H in the byte stream), bytes per paint, allocs per paint (mallocs from
the --profile stats file over external paints) and allocs per View() rebuild.
Then the top 15 functions from the CPU profile (go tool pprof -top).

Everything runs in a throwaway sandbox: HOME and every XDG root land in a
fresh temp dir, MTM_SANDBOX=1 is set, and the sandbox is deleted on exit —
the same sandbox semantics as contrib/sandbox-env.sh (reimplemented in
Python so the pty, the capability-answering wrapper and the metrics can live
in one place). The TUI is launched with --profile so the run is admitted
only when the process is inside that sandbox. Never touches real user data.
"""
import argparse
import json
import os
import pty
import fcntl
import shutil
import signal
import struct
import subprocess
import sys
import tempfile
import time
import select

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PAINT_MARK = b"\x1b[H"  # alt-screen flush starts with cursor-home
OSC_11_QUERY = b"\x1b]11;?\x1b\\"
OSC_11_ANSWER = b"\x1b]11;rgb:1c1c/1c1c/1c1c\x07"
DSR_QUERY = b"\x1b[6n"
DSR_ANSWER = b"\x1b[1;1R"

SCENARIOS = {
    # name → (ui yaml lines, timer_seed, keys)
    "home-ambient-off": (["    ambient: off"], False, []),
    "home-ambient-12fps": (
        ["    ambient: both", "    ambient_sidebar: true", "    ambient_fps: 12"],
        False,
        [],
    ),
    "focus-24fps": (
        ["    ambient: both", "    ambient_sidebar: true", "    ambient_fps: 24"],
        True,
        [("1", 0.5), ("F", 0.8)],  # list view, then focus the selected task
    ),
    "idle-ambient-5m": ([], False, []),  # shipped defaults (ambient both)
}


def seed(env, binary, data, config_ui_lines, timer_seed):
    subprocess.run([binary, "demo-seed", os.path.join(data, "projects")],
                   env=env, check=True, stdout=subprocess.DEVNULL)
    subprocess.run(["git", "init", "-q"], env=env, cwd=data, check=True)
    subprocess.run(["git", "add", "-A"], env=env, cwd=data, check=True)
    subprocess.run(["git", "-c", "user.name=mtm", "-c", "user.email=mtm@example.invalid",
                    "commit", "-qm", "demo: example tasks"],
                   env=env, cwd=data, check=True, stdout=subprocess.DEVNULL)
    ui_yaml = "\n".join(config_ui_lines)
    if ui_yaml:
        ui_yaml = "\nui:\n" + ui_yaml
    cfg_path = os.path.join(env["XDG_CONFIG_HOME"], "mtm", "config.yaml")
    os.makedirs(os.path.dirname(cfg_path), exist_ok=True)
    with open(cfg_path, "w") as f:
        f.write(f"data_repo: {data}\ndefault_project: inbox\nsync:\n    auto: false\n" + ui_yaml + "\n")
    if timer_seed:
        subprocess.run([binary, "timer", "start", "lt0001"],
                       env=env, check=True, stdout=subprocess.DEVNULL)


def run_scenario(name, binary, size, duration, settle, outdir):
    cols, rows = size
    sandbox = tempfile.mkdtemp(prefix="mtm-perf.")
    env = dict(os.environ)
    env.update({
        "HOME": sandbox + "/home",
        "XDG_DATA_HOME": sandbox + "/home/.local/share",
        "XDG_CONFIG_HOME": sandbox + "/home/.config",
        "XDG_CACHE_HOME": sandbox + "/home/.cache",
        "XDG_STATE_HOME": sandbox + "/home/.local/state",
        "OMARCHY_STATE_HOME": sandbox + "/home/.local/state/omarchy",
        "MTM_SANDBOX": "1",
        "TERM": "xterm-256color",
        "COLORTERM": "truecolor",
        "MTM_NO_SPLASH": "1",
        "GIT_TERMINAL_PROMPT": "0",
        "GIT_AUTHOR_NAME": "perf", "GIT_AUTHOR_EMAIL": "perf@example.invalid",
        "GIT_COMMITTER_NAME": "perf", "GIT_COMMITTER_EMAIL": "perf@example.invalid",
    })
    for d in ("home", "home/.local/share", "home/.config", "home/.cache",
              "home/.local/state", "home/.local/state/omarchy"):
        os.makedirs(os.path.join(sandbox, d), exist_ok=True)
    data = os.path.join(env["XDG_DATA_HOME"], "mtm")
    os.makedirs(data, exist_ok=True)
    ui_lines, timer_seed, keys = SCENARIOS[name]
    pprof_path = os.path.join(outdir, name + ".cpu.pprof")
    pid, master, rusage = None, None, None
    try:
        seed(env, binary, data, ui_lines, timer_seed)
        pid, master = pty.fork()
        if pid == 0:
            os.execve(binary, [binary, "tui", "--profile", pprof_path], env)
        fcntl.ioctl(master, 0x5414, struct.pack("HHHH", rows, cols, 0, 0))  # TIOCSWINSZ

        raw = b""
        start = time.monotonic()
        deadline = start + duration
        sent = []
        answered_osc = False
        answered_dsr = False
        while time.monotonic() < deadline:
            r, _, _ = select.select([master], [], [], 0.1)
            if r:
                try:
                    chunk = os.read(master, 65536)
                except OSError:
                    chunk = b""
                if not chunk:
                    break
                raw += chunk
                # Answer the terminal capability queries, like a real terminal.
                if not answered_osc and OSC_11_QUERY in raw:
                    os.write(master, OSC_11_ANSWER)
                    answered_osc = True
                if not answered_dsr and DSR_QUERY in raw:
                    os.write(master, DSR_ANSWER)
                    answered_dsr = True
            for key, at in keys:
                if (key, at) not in sent and time.monotonic() - start >= at:
                    os.write(master, key.encode())
                    sent.append((key, at))
        wall = time.monotonic() - start
        # Leave any mode we might be sitting in (focus esc, help esc, ...)
        # so Ctrl+C reaches the normal-mode handler, then quit.
        os.write(master, b"\x1b")
        time.sleep(0.4)
        os.write(master, b"\x03")  # Ctrl+C: bubbletea quits cleanly
        # The child blocks writing to the pty once its buffer fills, so the
        # quit key is only processed while we keep draining. Read until the
        # child exits (or 15s elapse), then force-kill.
        rusage = None
        exit_deadline = time.monotonic() + 15.0
        while time.monotonic() < exit_deadline:
            done, status, ru = os.wait4(pid, os.WNOHANG)
            if done == pid:
                rusage = ru
                break
            r, _, _ = select.select([master], [], [], 0.1)
            if r:
                try:
                    chunk = os.read(master, 65536)
                except OSError:
                    chunk = b""
                if not chunk:
                    continue
                raw += chunk
        if rusage is None:
            os.kill(pid, signal.SIGKILL)
            _, _, rusage = os.wait4(pid, 0)

        # Already drained while waiting for exit.
    finally:
        if pid is not None and rusage is None:
            try:
                os.kill(pid, signal.SIGKILL)
                os.wait4(pid, 0)
            except (ProcessLookupError, ChildProcessError):
                pass
        shutil.rmtree(sandbox, ignore_errors=True)

    paints = raw.count(PAINT_MARK)
    if rusage is None:
        class _ZeroRusage:  # child never exited and was killed
            ru_utime = 0.0
            ru_stime = 0.0
        rusage = _ZeroRusage()
    stats_path = pprof_path + ".stats.json"
    stats = {}
    try:
        with open(stats_path) as f:
            stats = json.load(f)
    except OSError as e:
        print(f"perf: warning: no stats file {stats_path}: {e}", file=sys.stderr)
    views = stats.get("views", 0)
    mallocs = stats.get("mallocs", 0)
    cpu_user = rusage.ru_utime
    cpu_sys = rusage.ru_stime
    cpu_pct = (cpu_user + cpu_sys) / wall * 100 if wall else 0.0
    return {
        "scenario": name, "wall": wall, "cpu_user": cpu_user, "cpu_sys": cpu_sys,
        "cpu_pct": cpu_pct, "pty_bytes": len(raw), "paints": paints,
        "paints_per_s": paints / wall if wall else 0.0,
        "bytes_per_paint": (len(raw) / paints) if paints else None,
        "views": views, "views_per_s": views / wall if wall else 0.0,
        "allocs_per_paint": (mallocs / paints) if paints else None,
        "allocs_per_view": (mallocs / views) if views else None,
        "view_bytes": stats.get("view_bytes", 0),
        "pprof": pprof_path, "stats": stats_path,
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--size", default="120x40")
    ap.add_argument("--duration", type=float, default=None)
    ap.add_argument("--settle", type=float, default=2.0)
    ap.add_argument("--out", default=os.environ.get("PERF_OUT",
                                                    tempfile.mkdtemp(prefix="mtm-perf-out.")))
    ap.add_argument("--binary", default=os.path.join(REPO, "bin", "mtm"))
    ap.add_argument("scenarios", nargs="*", default=list(SCENARIOS))
    args = ap.parse_args()
    w, _, h = args.size.lower().partition("x")
    size = (int(w), int(h))
    os.makedirs(args.out, exist_ok=True)
    results = []
    for name in args.scenarios:
        if name not in SCENARIOS:
            sys.exit(f"perf: unknown scenario {name!r} (want one of {', '.join(SCENARIOS)})")
        dur = args.duration
        if dur is None:
            dur = 300.0 if name == "idle-ambient-5m" else 30.0
        print(f"perf: scenario {name} ({size[0]}x{size[1]}, {dur:.0f}s) …", file=sys.stderr)
        results.append(run_scenario(name, args.binary, size, dur, args.settle, args.out))
    hdr = (f"{'scenario':<18} {'cpu%':>6} {'user_s':>7} {'sys_s':>6} {'wall_s':>6} "
           f"{'paints/s':>8} {'B/paint':>9} {'alloc/paint':>11} {'alloc/view':>10} {'views/s':>8}")
    print(hdr)
    print("-" * len(hdr))
    for r in results:
        bpp = f"{r['bytes_per_paint']:.0f}" if r["bytes_per_paint"] is not None else "n/a"
        app = f"{r['allocs_per_paint']:.0f}" if r["allocs_per_paint"] is not None else "n/a"
        apv = f"{r['allocs_per_view']:.1f}" if r["allocs_per_view"] is not None else "n/a"
        print(f"{r['scenario']:<18} {r['cpu_pct']:>5.1f}% {r['cpu_user']:>7.2f} {r['cpu_sys']:>6.2f} "
              f"{r['wall']:>6.1f} {r['paints_per_s']:>8.2f} {bpp:>9} {app:>11} {apv:>10} {r['views_per_s']:>8.2f}")
    summary = os.path.join(args.out, "perf-summary.json")
    with open(summary, "w") as f:
        json.dump(results, f, indent=2)
    print(f"perf: summary → {summary}")
    for r in results:
        print(f"\n== top 15 (flat) — {r['scenario']} ({r['pprof']}) ==")
        subprocess.run(["go", "tool", "pprof", "-top", "-nodecount=15", r["pprof"]],
                       check=False)


if __name__ == "__main__":
    main()
