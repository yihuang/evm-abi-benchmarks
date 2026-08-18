# evm-abi-benchmarks

Cross-language benchmarks for EVM ABI encoding/decoding: the **Lean** codec
in [`evm-abi-lean`](https://github.com/yihuang/evm-abi-lean) (pinned to
`bc14688`, the head of its #39 branch, in the lake manifest) against
**go-ethereum's `abi` package** (the mainstream Go ABI implementation).
Same shapes, same sizes, same µs/op methodology.

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
* The Lean rows are the `ValBA` runtime value family throughout — `encode`
  and `decodeStrict`, over packed `ByteArray` payloads — not the `List UInt8`
  specification the proofs are stated over, and not `Spec.encodeByteArray`,
  which is that specification's encoder run into a buffer.  Both are printed,
  but only the runtime one is keyed into the table: it is what the library
  recommends running, and timing the other against go-ethereum compared a Go
  encoder with a Lean *specification*.
* go-ethereum's `Pack`/`Unpack` go through reflection; that overhead is
  part of the real-world cost.

Absolute µs are machine-specific; the ratios are the robust claim.

## Numbers (Apple Silicon, one session)

| shape | Lean fast/ValBA | go-ethereum | Lean vs Go |
|---|---|---|---|
| encode flat `bytes[]` 500 | 77 | 165 | 2.1× ahead |
| encode flat 2000 | 328 | 569 | 1.7× ahead |
| encode `uint256[]` 1000 | 159 | 71 | 2.2× behind |
| encode nest depth 50 | 13 | 106 | 8.2× ahead |
| encode nest depth 200 | 55 | 1140 | 20.7× ahead |
| decode flat 500 (ValBA) | 53 | 72 | 1.4× ahead |
| decode flat 2000 (ValBA) | 223 | 268 | **parity** |
| decode `uint256[]` 2000 (ValBA) | 183 | 89 | 2.1× behind |
| encode unaligned 2000 | 369 | 550 | 1.5× ahead |
| decode unaligned 2000 (ValBA) | 256 | 303 | **parity** |
| encode `bytes32[]` 2000 | 151 | 180 | **parity** |
| decode `bytes32[]` 2000 (ValBA) | 172 | 144 | **parity** |

Both columns moved this time. On the decode side evm-abi-lean#40 finishes what
#39's `2^64` cap started: a length or offset word reads as four `UInt64` limbs
through `ba[i]` with an in-bounds proof rather than `ba[i]!`, and the reader is
`@[inline]`, so the `Option` it returns is never allocated. Decode flat 2000
goes 283 → 223 µs/op across those steps and flat 500 down to 53 — the first
decode row to clear the parity band outright.

On the encode side #41 skips a padding run when there is nothing to pad, which
is most of the time: `bytes32[]` pads by zero for every element, and its encode
drops 183 → 151.

`bytes32[]` *decode* holds at ~172 throughout, and that is the control — it
reads one length word for a whole array rather than one per element, so it is
the row that should not move, and does not.

`b5eb012` makes an ABI word four `UInt64` limbs rather than a `Nat`
(yihuang/lean-binary#5 underneath it): `ValBA (.uint m)` carries a `UInt256`,
so encoding and decoding one no longer builds a GMP integer.  That is the
`uint256[]` rows — encode 525→159, decode 1945→183 µs/op, taking them from
6.2× and 18.2× behind go-ethereum to 2.2× and 2.1×.

The other rows moved because *this repo* was measuring the wrong encoder.
`benchTy` keyed `Spec.encodeByteArray` over `Ty.Val` — the specification, whose
payloads are `List UInt8` — while every decode row keyed `decodeStrict` over
`ValBA`.  Keying the runtime encoder instead is worth 5× on flat `bytes[]`
with no library change at all, and turns "2.3× behind" there into 2.1× ahead.
Nothing about the library changed; the benchmark had been comparing a Go
encoder against a Lean specification.

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
