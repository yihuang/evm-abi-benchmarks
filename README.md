# evm-abi-benchmarks

Cross-language benchmarks for EVM ABI encoding/decoding: the **Lean** codec
in [`evm-abi-lean`](https://github.com/yihuang/evm-abi-lean) (pinned to
`main` in the lake manifest) against **go-ethereum's `abi` package** (the
mainstream Go ABI implementation).  Same shapes, same sizes, same µs/op
methodology.

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
```

Both print sizes; Go's sizes are Lean + 32 (its `Arguments.Pack` wraps a
single top-level dynamic argument in an offset word — the bytes are
otherwise identical).

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
| encode flat `bytes[]` 500 | 633 | 175 | 3.6× behind |
| encode flat 2000 | 2577 | 702 | 3.7× behind |
| encode `uint256[]` 1000 | 716 | 86 | 8.3× behind |
| encode nest depth 50 | 83 | 125 | **1.5× ahead** |
| encode nest depth 200 | 316 | 1881 | **6.0× ahead** |
| decode flat 500 (ValBA) | 77 | 93 | **parity** |
| decode flat 2000 (ValBA) | 319 | 369 | **parity** |
| encode unaligned 2000 | 1230 | 587 | 2.1× behind |
| decode unaligned 2000 (ValBA) | 362 | 358 | **parity** |
| encode `bytes32[]` 2000 | 199 | 211 | **parity** |
| decode `bytes32[]` 2000 (ValBA) | 199 | 183 | **parity** |

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
