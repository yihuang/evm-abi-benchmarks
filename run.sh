#!/usr/bin/env bash
# Build and run both benchmarks, then show the cross-language table.
set -e
cd "$(dirname "$0")"

echo "=== Lean ==="
(cd lean && lake build bench && ./.lake/build/bin/bench)

echo
echo "=== Go (go-ethereum) ==="
(cd go && go build -o bench . && ./bench)
