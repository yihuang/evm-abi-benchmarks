# evm-abi-benchmarks

Cross-language benchmarks for EVM ABI encoding/decoding: the **Lean** codec
in [`evm-abi-lean`](https://github.com/yihuang/evm-abi-lean) (pinned to the
`monad-walkers` branch in the lake manifest — `main` plus a `@[csimp]`
decoder fast path) against **go-ethereum's `abi` package** (the mainstream
Go ABI implementation).  Same shapes, same sizes, same µs/op methodology.

```
lean/   Bench.lean — the Lean benchmark (lake project depending on evm-abi-lean)
go/     main.go   — the go-ethereum mirror (go-ethereum v1.13.14)
```

## Run

```bash
# Lean — fetches evm-abi-lean (rev pinned in lean/lake-manifest.json), then builds and runs
cd lean && lake build bench && ./.lake/build/bin/bench

# Go
cd go && go build -o bench . && ./bench

# both, then print the Lean-vs-Go table below
./scripts/bench_diff.py
```

Both print sizes; Go's sizes are Lean + 32 (its `Arguments.Pack` wraps a
single top-level dynamic argument in an offset word — the bytes are
otherwise identical).

The comparable rows are also emitted as machine-readable
`BENCH <key> <µs/op> <bytes>` lines (the same keys on both sides);
`scripts/bench_diff.py` runs both binaries, joins them on the key, and
regenerates the table below.  Pass two captured outputs as arguments
instead to diff without re-running (`./scripts/bench_diff.py lean.txt go.txt`).

## Methodology

* Both binaries compiled, run on the same machine, minutes apart.
* 20 reps per case, reported as µs/op.
* The Lean rows: the *executable* codec — `Spec.encodeByteArray`,
  `decodeStrictBA` (list cursors), and the `ValBA` runtime value family
  (`encode` / `decodeStrict`, packed `ByteArray` payloads) — not the
  `List UInt8` specification, which is what the proofs are stated over.
* go-ethereum's `Pack`/`Unpack` go through reflection; that overhead is
  part of the real-world cost.

Absolute µs are machine-specific; the ratios are the robust claim.

## Numbers (Apple Silicon, one session)

| shape | Lean fast/ValBA | go-ethereum | Lean vs Go |
|---|---|---|---|
| encode flat `bytes[]` 500 | 492 | 198 | 2.5× behind |
| encode flat 2000 | 2103 | 752 | 2.8× behind |
| encode `uint256[]` 1000 | 514 | 90 | 5.7× behind |
| encode nest depth 50 | 59 | 130 | 2.2× ahead |
| encode nest depth 200 | 243 | 1658 | 6.8× ahead |
| decode flat 500 (ValBA) | 63 | 89 | 1.4× ahead |
| decode flat 2000 (ValBA) | 264 | 324 | **parity** |
| decode `uint256[]` 2000 (ValBA) | 1821 | 103 | 17.7× behind |
| encode unaligned 2000 | 808 | 599 | 1.3× behind |
| decode unaligned 2000 (ValBA) | 293 | 339 | **parity** |
| encode `bytes32[]` 2000 | 194 | 191 | **parity** |
| decode `bytes32[]` 2000 (ValBA) | 133 | 153 | **parity** |

(`monad-walkers` is `main` plus one commit — `decodeBAValFast`, a
`@[csimp]` copy of `decodeBAVal` whose array/tuple walkers are replaced by
cursor-threaded loops.  Against the same-session `main` binary, the decode
rows are ~20-35% faster on cheap elements — flat `bytes[]` 85→64, flat
2000 327→257, unaligned 351→298, `bytes32[]` 207→135 µs/op — and
unchanged on `uint256[]` (1905→1854), where each element is dominated by
bignum word decoding.)

## What the shapes test

* **flat `bytes[]`** — constant factor.  Lean's input is a `List UInt8`
  (cons cell per byte), so the encode is a per-byte push; go-ethereum
  copies `[]byte` slices.
* **`uint256[]`** — full-width bignum words.  Lean's `Nat` arithmetic
  (chunked 8 bytes per bignum op) vs Go's C-level `big.Int`.
* **nested tuples** `(bytes, (bytes, …))` — asymptotics.  go-ethereum's
  `pack` re-appends the tail at every level (`O(n·d)`); Lean's builder is
  an `O(1)` append, so it stays linear and pulls ahead with depth.
* **decode** — the `ValBA` rows use packed `ByteArray` payloads (one
  `extract` per payload); go-ethereum slices the input.  Both are at
  parity, and both are ~13× faster than Lean's `List UInt8` decode.
* **unaligned 100-byte payloads** — the zero padding (28 bytes per element)
  is checked by index (`allZerosBA`) on the Lean side, no list built.
* **`bytes32[]`** — fixed-word payloads; the bounded-list round trip.
