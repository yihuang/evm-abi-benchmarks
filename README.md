# evm-abi-benchmarks

Cross-language benchmarks for EVM ABI encoding/decoding: the **Lean** codec
in [`evm-abi-lean`](https://github.com/yihuang/evm-abi-lean) (its `word-leaf`
branch, over lean-binary's `pushlimb`) against **go-ethereum's `abi` package**
(the mainstream Go ABI implementation).  Same shapes, same sizes, same µs/op
methodology.

```
lean/   Bench.lean — the Lean benchmark (lake project depending on evm-abi-lean)
go/     main.go   — the go-ethereum mirror (go-ethereum v1.13.14)
```

## Run

```bash
# Lean — fetches evm-abi-lean (resolved in lean/lake-manifest.json), builds and runs
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

`lean/lakefile.toml` tracks the `word-leaf` branch, which pins lean-binary to
`pushlimb`; run `lake update` to move to the branch tips.

## Methodology

* Both binaries compiled, run on the same machine, minutes apart.
* 20 reps per case, reported as µs/op; each table cell is the median of ten
  full `./scripts/bench_diff.py` runs.
* The Lean column is the stable one — it varies by ~2–4% across those runs,
  where go-ethereum's spans ~10–20%.  Rows sitting near the 1.25× parity band
  can therefore change verdict on the Go column alone, with nothing on the
  Lean side moving.
* The Lean rows are the `ValBA` runtime value family throughout — `encode`
  and `decodeStrict`, over packed `ByteArray` payloads — not the `List UInt8`
  specification the proofs are stated over.  The bench prints both, but only
  the runtime one is keyed into the table: timing the specification against
  go-ethereum would compare a Go encoder with a Lean *specification*.
* go-ethereum's `Pack`/`Unpack` go through reflection; that overhead is
  part of the real-world cost.

Absolute µs are machine-specific; the ratios are the robust claim.

## Numbers (Apple Silicon, median of ten runs)

| shape | Lean fast/ValBA | go-ethereum | Lean vs Go |
|---|---|---|---|
| encode flat `bytes[]` 500 | 70 | 148 | 2.1× ahead |
| encode flat 2000 | 297 | 594 | 2.0× ahead |
| encode `uint256[]` 1000 | 35 | 71 | 2.0× ahead |
| encode nest depth 50 | 12 | 108 | 9.0× ahead |
| encode nest depth 200 | 50 | 1184 | 23.7× ahead |
| decode flat 500 (ValBA) | 55 | 68 | **parity** |
| decode flat 2000 (ValBA) | 224 | 275 | **parity** |
| decode `uint256[]` 2000 (ValBA) | 63 | 82 | 1.3× ahead |
| encode unaligned 2000 | 339 | 447 | 1.3× ahead |
| decode unaligned 2000 (ValBA) | 249 | 286 | **parity** |
| encode `bytes32[]` 2000 | 148 | 171 | **parity** |
| decode `bytes32[]` 2000 (ValBA) | 170 | 144 | **parity** |

Four things earn those rows.  The builder appends in `O(1)`, so nesting stays
linear where go-ethereum's `pack` re-appends the tail at every level.  An ABI
word is four `UInt64` limbs rather than a `Nat`, so no word round-trips through
a bignum.  The `uint` array arms are fused: the length word and every element
go straight into the pre-sized buffer, instead of building a part per element
and walking them afterwards.  And words are pushed a limb at a time, so length
and offset words stop recopying the bytes already written.

Faster word *arithmetic* would not show up here.  The codec only ever converts
words — never computes with them — so rewriting `UInt256`'s bitwise and
arithmetic operations to work on limbs, worth 55–170× in lean-binary's own
bench, moves every row in this table by less than noise.

## What the shapes test

* **flat `bytes[]`** — constant factor.  Lean's input is a `List UInt8`
  (cons cell per byte), so the encode is a per-byte push; go-ethereum
  copies `[]byte` slices.
* **`uint256[]`** — full-width words, limbs on the Lean side vs Go's C-level
  `big.Int`.
* **nested tuples** `(bytes, (bytes, …))` — asymptotics.  go-ethereum's
  `pack` is `O(n·d)` in the depth; Lean's builder stays linear and pulls
  ahead as depth grows.
* **decode** — the `ValBA` rows use packed `ByteArray` payloads (one
  `extract` per payload); go-ethereum slices the input.  Both are at or just
  ahead of parity, and both are ~18× faster than Lean's `List UInt8` decode.
* **unaligned 100-byte payloads** — the zero padding (28 bytes per element)
  is checked by index (`allZerosBA`) on the Lean side, no list built.
* **`bytes32[]`** — fixed-word payloads; the bounded-list round trip.  Its
  decode reads one length word for a whole array rather than one per element,
  which makes it the control: the row that should not move, and does not.
