package main

// Cross-language ABI benchmark: go-ethereum's `abi` package (the mainstream
// Go ABI implementation) against the Lean codec in evm-abi-lean's `bytes`
// branch.  Same shapes, same sizes, same µs/op methodology (20 reps) as the
// Lean `Bench.lean` in ../lean.
//
// One systematic size difference: `Arguments.Pack` wraps a single top-level
// dynamic argument in a 32-byte offset word, so Go's encodings are Lean + 32
// bytes (160064 vs 160032, etc.).  The bytes are otherwise identical.

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

const reps = 20

func timeIt(label string, act func() int) {
	t0 := time.Now()
	checksum := 0
	for i := 0; i < reps; i++ {
		checksum += act()
	}
	t1 := time.Now()
	us := float64(t1.Sub(t0).Nanoseconds()) / 1000 / float64(reps)
	fmt.Printf("  %s: %6.0f us/op  (%d bytes)\n", label, us, checksum/reps)
}

// mkBytes is the 256-byte payload used by the nested-tuple values
// (nest.gen.go), matching abi-lean's mkBytes 256.
func mkBytes() []byte { return mkBytesOf(256) }

// mkBytesOf is a payload-byte value filled with 7, matching abi-lean's
// mkBytes.
func mkBytesOf(payload int) []byte {
	b := make([]byte, payload)
	for i := range b {
		b[i] = 7
	}
	return b
}

// flatValOf is a flat bytes[] with payload-byte elements.
func flatValOf(payload, n int) [][]byte {
	v := make([][]byte, n)
	for i := range v {
		v[i] = mkBytesOf(payload)
	}
	return v
}

// bytes32Val is a bytes32[] with full 32-byte words, matching the Lean
// bytesN case.
func bytes32Val(n int) [][32]byte {
	v := make([][32]byte, n)
	for i := range v {
		for j := range v[i] {
			v[i][j] = 7
		}
	}
	return v
}

// wide is 0x123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0
var wide = func() *big.Int {
	x, ok := new(big.Int).SetString("123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0", 16)
	if !ok {
		panic("bad hex")
	}
	return x
}()

func wideVal(n int) []*big.Int {
	v := make([]*big.Int, n)
	for i := range v {
		v[i] = wide
	}
	return v
}

// nestComponents builds the ABI components for (bytes, (bytes, (...))), k deep.
func nestComponents(k int) []abi.ArgumentMarshaling {
	if k == 0 {
		return []abi.ArgumentMarshaling{{Name: "a", Type: "bytes"}}
	}
	return []abi.ArgumentMarshaling{
		{Name: "a", Type: "bytes"},
		{Name: "b", Type: "tuple", Components: nestComponents(k - 1)},
	}
}

func nestType(k int) (abi.Type, error) {
	return abi.NewType("tuple", "", nestComponents(k))
}

func benchTy(label string, ty abi.Type, v any) {
	fmt.Println(label)
	args := abi.Arguments{{Type: ty}}
	timeIt("go-ethereum Pack       ", func() int {
		out, err := args.Pack(v)
		if err != nil {
			panic(err)
		}
		return len(out)
	})
}

func benchDecode(label string, args abi.Arguments, data []byte) {
	fmt.Printf("%s (%d bytes)\n", label, len(data))
	timeIt("go-ethereum Unpack     ", func() int {
		if _, err := args.Unpack(data); err != nil {
			panic(err)
		}
		return len(data)
	})
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	flatTy, err := abi.NewType("bytes[]", "", nil)
	must(err)
	flatArgs := abi.Arguments{{Type: flatTy}}
	wideTy, err := abi.NewType("uint256[]", "", nil)
	must(err)
	bytes32Ty, err := abi.NewType("bytes32[]", "", nil)
	must(err)
	bytes32Args := abi.Arguments{{Type: bytes32Ty}}

	fmt.Println("== flat bytes[], 256-byte elements (constant factor) ==")
	benchTy("-- 500 elements", flatTy, flatValOf(256, 500))
	benchTy("-- 2000 elements", flatTy, flatValOf(256, 2000))
	fmt.Println("== uint256[], full-width values (bignum word encoding) ==")
	benchTy("-- 1000 words", wideTy, wideVal(1000))
	fmt.Println("== nested tuples (bytes, (bytes, ...)) (asymptotics) ==")
	for _, d := range []int{50, 200} {
		nt, err := nestType(d)
		must(err)
		benchTy(fmt.Sprintf("-- depth %d", d), nt, nestValFor(d))
	}
	fmt.Println("== decode: flat bytes[] ==")
	enc500, err := flatArgs.Pack(flatValOf(256, 500))
	must(err)
	enc2000, err := flatArgs.Pack(flatValOf(256, 2000))
	must(err)
	benchDecode("-- 500 elements", flatArgs, enc500)
	benchDecode("-- 2000 elements", flatArgs, enc2000)
	fmt.Println("== unaligned 100-byte payloads (pad 28) ==")
	encUnal, err := flatArgs.Pack(flatValOf(100, 2000))
	must(err)
	benchTy("-- 2000 elements, encode", flatTy, flatValOf(100, 2000))
	benchDecode("-- 2000 elements, decode", flatArgs, encUnal)
	fmt.Println("== bytes32[]: fixed-word payloads ==")
	encB32, err := bytes32Args.Pack(bytes32Val(2000))
	must(err)
	benchTy("-- 2000 elements, encode", bytes32Ty, bytes32Val(2000))
	benchDecode("-- 2000 elements, decode", bytes32Args, encB32)

	// size cross-check against the Lean bench (Lean sizes are Go minus the
	// 32-byte argument-wrapper offset)
	fmt.Println("== size cross-check (Lean sizes, from the Go runs) ==")
	fmt.Printf("flat 500:    %d\n", len(enc500)-32)
	fmt.Printf("flat 2000:   %d\n", len(enc2000)-32)
	encWide, err := abi.Arguments{{Type: wideTy}}.Pack(wideVal(1000))
	must(err)
	fmt.Printf("uint256[]:   %d\n", len(encWide)-32)
	for _, d := range []int{50, 200} {
		nt, err := nestType(d)
		must(err)
		enc, err := abi.Arguments{{Type: nt}}.Pack(nestValFor(d))
		must(err)
		fmt.Printf("nest %d:     %d\n", d, len(enc)-32)
	}
	fmt.Printf("unaligned:   %d\n", len(encUnal)-32)
	fmt.Printf("bytes32[]:   %d\n", len(encB32)-32)
}

func nestValFor(d int) any {
	switch d {
	case 50:
		return nestVal50()
	case 200:
		return nestVal200()
	}
	panic("unexpected depth")
}
