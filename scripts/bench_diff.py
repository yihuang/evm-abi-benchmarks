#!/usr/bin/env python3
"""Build & run both benchmarks and print the cross-language diff table (the
numbers that go in the README "Numbers" section).

Usage:
    ./bench_diff.py                  # build & run both benches, then diff
    ./bench_diff.py lean.txt go.txt  # diff previously captured outputs

Both benches emit `BENCH <key> <µs/op> <bytes>` lines for the comparable
rows (the same keys on both sides); this script joins them on the key and
classifies each ratio as behind / parity / ahead.
"""

import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

# |lean/go - 1| below this ⇒ parity; above it the multiple is reported.
PARITY_BAND = 1.25

# Rows of the README table, in order: (BENCH key, human label).
ROWS = [
    ("encode/flat/500", "encode flat `bytes[]` 500"),
    ("encode/flat/2000", "encode flat 2000"),
    ("encode/uint256/1000", "encode `uint256[]` 1000"),
    ("encode/nest/50", "encode nest depth 50"),
    ("encode/nest/200", "encode nest depth 200"),
    ("decode/flat/500", "decode flat 500 (ValBA)"),
    ("decode/flat/2000", "decode flat 2000 (ValBA)"),
    ("encode/unaligned/2000", "encode unaligned 2000"),
    ("decode/unaligned/2000", "decode unaligned 2000 (ValBA)"),
    ("encode/bytes32/2000", "encode `bytes32[]` 2000"),
    ("decode/bytes32/2000", "decode `bytes32[]` 2000 (ValBA)"),
]

BENCH_RE = re.compile(r"^BENCH (\S+) (\d+) (\d+)$")


def parse(text: str) -> dict:
    """Parse `BENCH <key> <us/op> <bytes>` lines into {key: (us, bytes)}."""
    out = {}
    for line in text.splitlines():
        m = BENCH_RE.match(line)
        if m:
            out[m.group(1)] = (int(m.group(2)), int(m.group(3)))
    return out


def verdict(lean_us: int, go_us: int) -> str:
    ratio = lean_us / go_us
    if ratio > PARITY_BAND:
        return f"{ratio:.1f}× behind"
    if ratio < 1 / PARITY_BAND:
        return f"{1 / ratio:.1f}× ahead"
    return "**parity**"


def run(cmd: list[str]) -> str:
    """Run a command in ROOT, print stdout live, return it."""
    proc = subprocess.run(cmd, cwd=ROOT, capture_output=True, text=True)
    sys.stdout.write(proc.stdout)
    sys.stdout.flush()
    if proc.returncode != 0:
        sys.stderr.write(proc.stderr)
        sys.exit(f"{' '.join(cmd)} failed ({proc.returncode})")
    return proc.stdout


def main() -> None:
    if len(sys.argv) == 3:
        lean = parse(Path(sys.argv[1]).read_text())
        go = parse(Path(sys.argv[2]).read_text())
    else:
        print("=== Lean ===")
        lean = parse(run(["bash", "-lc", "cd lean && lake build bench && ./.lake/build/bin/bench"]))
        print("\n=== Go (go-ethereum) ===")
        go = parse(run(["bash", "-lc", "cd go && go build -o bench . && ./bench"]))

    missing = [k for k, _ in ROWS if k not in lean] + [k for k, _ in ROWS if k not in go]
    if missing:
        sys.exit(f"missing BENCH rows: {missing}")

    print("\n| shape | Lean fast/ValBA | go-ethereum | Lean vs Go |")
    print("|---|---|---|---|")
    for key, label in ROWS:
        lu, _ = lean[key]
        gu, _ = go[key]
        print(f"| {label} | {lu} | {gu} | {verdict(lu, gu)} |")


if __name__ == "__main__":
    main()
