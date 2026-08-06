# evm-abi-benchmarks

Cross-language benchmarks for EVM ABI encoding/decoding: the **Lean** codec
in [`evm-abi-lean`](https://github.com/yihuang/evm-abi-lean) (pinned to
`4222e07`, the head of its #39 branch, in the lake manifest) against
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
| encode flat `bytes[]` 500 | 85 | 160 | 1.9× ahead |
| encode flat 2000 | 364 | 618 | 1.7× ahead |
| encode `uint256[]` 1000 | 156 | 73 | 2.1× behind |
| encode nest depth 50 | 14 | 100 | 7.1× ahead |
| encode nest depth 200 | 58 | 1255 | 21.6× ahead |
| decode flat 500 (ValBA) | 59 | 71 | **parity** |
| decode flat 2000 (ValBA) | 234 | 278 | **parity** |
| decode `uint256[]` 2000 (ValBA) | 177 | 76 | 2.3× behind |
| encode unaligned 2000 | 359 | 459 | 1.3× ahead |
| decode unaligned 2000 (ValBA) | 264 | 301 | **parity** |
| encode `bytes32[]` 2000 | 183 | 173 | **parity** |
| decode `bytes32[]` 2000 (ValBA) | 174 | 147 | **parity** |

The decode rows moved; the encode rows did not. evm-abi-lean#39 caps dynamic
payload lengths at `2^64`, and spends that cap twice: a length or offset word
reads as four `UInt64` limbs rather than accumulating 32 bytes into a `Nat`,
and those reads take their in-bounds proof as an argument, so the bytes come
out through `ba[i]` with no bounds test and no panic branch. Decode flat 2000
goes 283 → 260 → 234 µs/op across the two steps, unaligned 313 → 291 → 264 —
both now on the faster side of parity with go-ethereum. The second step was
worth more than the first.

`bytes32[]` reads one length word for a whole array and holds at ~175
throughout: that control is what makes the rest signal. The cap alone measures
as nothing, and so do the encode rows and the whole go-ethereum column —
nothing on either reads a length word, so their drift between sessions is not
a result.

`b5eb012` makes an ABI word four `UInt64` limbs rather than a `Nat`
(yihuang/lean-binary#5 underneath it): `ValBA (.uint m)` carries a `UInt256`,
so encoding and decoding one no longer builds a GMP integer.  That is the
`uint256[]` rows — encode 525→156, decode 1945→177 µs/op, taking them from
6.2× and 18.2× behind go-ethereum to 2.1× and 2.3×.

The other rows moved because *this repo* was measuring the wrong encoder.
`benchTy` keyed `Spec.encodeByteArray` over `Ty.Val` — the specification, whose
payloads are `List UInt8` — while every decode row keyed `decodeStrict` over
`ValBA`.  Keying the runtime encoder instead is worth 5× on flat `bytes[]`
with no library change at all, and turns "2.3× behind" there into 1.9× ahead.
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
