#!/usr/bin/env python3
"""snap.py — run a command in a pty inside the shared mtm sandbox and print
the final screen (via pyte) or the raw byte stream.

Usage: snap.py [--size WxH] [--timeout SECONDS] [--settle SECONDS] [--keys STR] [--raw] <cmd> [args...]

  --size    pty size, default 100x30
  --timeout kill the child after this many seconds, default 15 (exit 1)
  --settle  seconds to wait before sending --keys, default 2
  --keys    bytes to write to the pty after --settle (e.g. "q" to quit the TUI)
  --raw     print the raw pty bytes instead of the pyte-rendered screen

The child runs as: bash -c 'source "<repo>/contrib/sandbox-env.sh"; "$@"' —
the same sandbox contrib/demo.sh and contrib/smoke.sh source, so the child
and everything it launches (e.g. make demo -> contrib/demo.sh) all land in
the one throwaway sandbox pointed to by MTM_SANDBOX_DIR. The sandbox is
deleted when the child exits; snap.py removes any residue afterwards.
"""
import fcntl
import os
import pty
import select
import shutil
import signal
import struct
import sys
import tempfile
import time


def emit(buf, cols, rows, raw):
    if raw:
        sys.stdout.buffer.write(buf)
        return
    try:
        import pyte
    except ImportError:
        sys.stdout.buffer.write(buf)
        return
    screen = pyte.Screen(columns=cols, lines=rows)
    pyte.ByteStream(screen).feed(buf)
    print("\n".join(line.rstrip() for line in screen.display))


def parse_size(s):
    w, sep, h = s.lower().partition("x")
    if not sep or not w.isdigit() or not h.isdigit():
        sys.exit(f"snap: invalid --size {s!r} (want 100x30)")
    return int(w), int(h)


def main():
    args = sys.argv[1:]
    size, timeout, settle, keys, raw = "100x30", 15.0, 2.0, None, False
    while args and args[0].startswith("--"):
        flag, _, value = args.pop(0).partition("=")
        if not value and args and not args[0].startswith("--") and flag in ("--size", "--timeout", "--settle", "--keys"):
            value = args.pop(0)
        if flag == "--size":
            size = value
        elif flag == "--timeout":
            timeout = float(value)
        elif flag == "--settle":
            settle = float(value)
        elif flag == "--keys":
            keys = value
        elif flag == "--raw":
            raw = True
        else:
            sys.exit(f"snap: unknown flag {flag}")
    if not args:
        sys.exit(__doc__)
    cols, rows = parse_size(size)

    root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    helper = os.path.join(root, "contrib", "sandbox-env.sh")

    sandbox = tempfile.mkdtemp(prefix="mtm-sandbox.")
    # A dumb pty never answers capability queries (OSC 11 background-color,
    # ESC[6n cursor position), so bubbletea would wait forever before its
    # first render. Real terminals answer; TERM=dumb makes it skip asking.
    env = dict(os.environ, MTM_SANDBOX_DIR=sandbox, TERM="dumb")
    argv = ["bash", "-c", 'source "$1"; shift; "$@"', "bash", helper, *args]

    pid, master = pty.fork()
    if pid == 0:
        os.execvpe("bash", argv, env)

    fcntl.ioctl(master, 0x5414, struct.pack("HHHH", rows, cols, 0, 0))  # TIOCSWINSZ

    buf = b""
    start = time.monotonic()
    sent = keys is None
    deadline = start + timeout
    while True:
        now = time.monotonic()
        if not sent and now - start >= settle:
            # Send one byte at a time: bubbletea's key parser drops a whole
            # multi-byte run written in a single write (e.g. "2D"), so a
            # scripted sequence needs to look like a human typing.
            for ch in keys:
                os.write(master, ch.encode())
                time.sleep(0.08)
            sent = True
        if now >= deadline:
            os.kill(pid, signal.SIGKILL)
            os.waitpid(pid, 0)
            shutil.rmtree(sandbox, ignore_errors=True)
            emit(buf, cols, rows, raw)
            sys.exit(f"snap: timed out after {timeout}s")
        r, _, _ = select.select([master], [], [], 0.1)
        if r:
            try:
                chunk = os.read(master, 65536)
            except OSError:
                chunk = b""
            if not chunk:
                break
            buf += chunk
        else:
            done, status = os.waitpid(pid, os.WNOHANG)
            if done == pid:
                # drain whatever is left
                while True:
                    r, _, _ = select.select([master], [], [], 0.1)
                    if not r:
                        break
                    try:
                        buf += os.read(master, 65536)
                    except OSError:
                        break
                break
    os.waitpid(pid, 0)
    shutil.rmtree(sandbox, ignore_errors=True)  # no-op if the helper's trap already cleaned it
    removed = not os.path.exists(sandbox)

    emit(buf, cols, rows, raw)
    print(f"snap: sandbox {'removed' if removed else 'STILL PRESENT'}: {sandbox}", file=sys.stderr)
    if not removed:
        sys.exit(1)


if __name__ == "__main__":
    main()
