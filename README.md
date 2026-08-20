# evm-abi-benchmarks

Cross-language benchmarks for EVM ABI encoding/decoding: the **Lean** codec
in [`evm-abi-lean`](https://github.com/yihuang/evm-abi-lean) (pinned to
`689884b`, its `main` after #42, in the lake manifest) against
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
* Each table cell is the median of three full `./scripts/bench_diff.py` runs.
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

## Numbers (Apple Silicon, median of three runs)

| shape | Lean fast/ValBA | go-ethereum | Lean vs Go |
|---|---|---|---|
| encode flat `bytes[]` 500 | 79 | 144 | 1.8× ahead |
| encode flat 2000 | 326 | 586 | 1.8× ahead |
| encode `uint256[]` 1000 | 162 | 78 | 2.1× behind |
| encode nest depth 50 | 13 | 110 | 8.5× ahead |
| encode nest depth 200 | 55 | 1151 | 20.9× ahead |
| decode flat 500 (ValBA) | 56 | 67 | **parity** |
| decode flat 2000 (ValBA) | 223 | 277 | **parity** |
| decode `uint256[]` 2000 (ValBA) | 182 | 83 | 2.2× behind |
| encode unaligned 2000 | 372 | 448 | **parity** |
| decode unaligned 2000 (ValBA) | 254 | 279 | **parity** |
| encode `bytes32[]` 2000 | 150 | 174 | **parity** |
| decode `bytes32[]` 2000 (ValBA) | 169 | 147 | **parity** |

The pin sits on `main` (`689884b`, evm-abi-lean#42) and the toolchain on
v4.33.0.  An apparent +5–9% on `encode uint256[] 1000` across the toolchain
bump traced to binary layout of the runtime archive, not to any code change —
the hot path compiles identically under both toolchains — and it does not
reproduce across machine states, so treat that row as ±5% between sessions.

What follows is the history, each entry against the table current when it
landed. On the decode side evm-abi-lean#40 finishes what
#39's `2^64` cap started: a length or offset word reads as four `UInt64` limbs
through `ba[i]` with an in-bounds proof rather than `ba[i]!`, and the reader is
`@[inline]`, so the `Option` it returns is never allocated. Decode flat 2000
goes 283 → 223 µs/op across those steps and flat 500 down to 56.

On the encode side #41 skips a padding run when there is nothing to pad, which
is most of the time: `bytes32[]` pads by zero for every element, and its encode
drops 183 → 150.

`bytes32[]` *decode* holds at ~170 throughout, and that is the control — it
reads one length word for a whole array rather than one per element, so it is
the row that should not move, and does not.

**Measured negative: the limb-native word operations do not reach these rows.**
lean-binary#7 rewrote `UInt256`'s `and`/`or`/`xor`/`not`/`add`/`sub`/`mul` to
work on four `UInt64` limbs instead of going through `BitVec` — 170× on the
bitwise ops, 64× on `add`, 55× on `mul` in that library's own bench.  Re-pinning
across it (`69b3f96` → `957b5bd`, which carries `binary` `5b6b371` → `c4f8f88`)
moves every row here by less than 8%, scattered in both directions, with both
`bytes32[]` controls flat.  That is noise, and the reason is structural: the ABI
codec only ever *converts* words — `toNat`, `ofNat`, `toBEByteArrayFast`,
`ofBEByteArrayAt` — and never computes with them, so faster arithmetic has
nothing here to make faster.  The table is unchanged; the pin moved for
consistency with what the rest of the tree builds against, not for a number.

`b5eb012` makes an ABI word four `UInt64` limbs rather than a `Nat`
(yihuang/lean-binary#5 underneath it): `ValBA (.uint m)` carries a `UInt256`,
so encoding and decoding one no longer builds a GMP integer.  That is the
`uint256[]` rows — encode 525→162, decode 1945→182 µs/op, taking them from
6.2× and 18.2× behind go-ethereum to 2.1× and 2.2×.

The other rows moved because *this repo* was measuring the wrong encoder.
`benchTy` keyed `Spec.encodeByteArray` over `Ty.Val` — the specification, whose
payloads are `List UInt8` — while every decode row keyed `decodeStrict` over
`ValBA`.  Keying the runtime encoder instead is worth 5× on flat `bytes[]`
with no library change at all, and turns "2.3× behind" there into 1.8× ahead.
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
  `extract` per payload); go-ethereum slices the input.  Both are at or just
  ahead of parity, and both are ~18× faster than Lean's `List UInt8` decode.
* **unaligned 100-byte payloads** — the zero padding (28 bytes per element)
  is checked by index (`allZerosBA`) on the Lean side, no list built.
* **`bytes32[]`** — fixed-word payloads; the bounded-list round trip.
