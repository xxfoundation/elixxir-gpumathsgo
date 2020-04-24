////////////////////////////////////////////////////////////////////////////////
// Copyright © 2019 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

//+build linux,cuda

package gpumaths

import (
	"gitlab.com/elixxir/crypto/cryptops"
	"gitlab.com/elixxir/crypto/cyclic"
	"math/rand"
	"testing"
)

// Helper functions shared by tests are located in gpu_test.go

func initExp() *cyclic.Group {
	return initTestGroup()
}

func expCPU(t testing.TB, batchSize uint32, grp *cyclic.Group,
	x, y, z *cyclic.IntBuffer) {
	for i := uint32(0); i < batchSize; i++ {
		cryptops.Exp(grp, x.Get(i), y.Get(i), z.Get(i))
	}
}

func expGPU(t testing.TB, streamPool *StreamPool, batchSize uint32,
	grp *cyclic.Group, x, y, z *cyclic.IntBuffer) {
	_, err := ExpChunk(streamPool, grp, x, y, z)
	if err != nil {
		t.Fatal(err)
	}
}

// Runs precomp decrypt test with GPU stream pool and graphs
func TestExp(t *testing.T) {
	batchSize := uint32(1024)
	grp := initExp()

	x := grp.NewIntBuffer(batchSize, grp.NewInt(1))
	initRandomIntBuffer(grp, batchSize, x, 42)
	y := grp.NewIntBuffer(batchSize, grp.NewInt(1))
	initRandomIntBuffer(grp, batchSize, y, 43)

	zCPU := grp.NewIntBuffer(batchSize, grp.NewInt(1))
	zGPU := grp.NewIntBuffer(batchSize, grp.NewInt(1))

	// Run CPU
	expCPU(t, batchSize, grp, x, y, zCPU)

	// Run GPU
	streamPool, err := NewStreamPool(2, 65536)
	if err != nil {
		t.Fatal(err)
	}
	expGPU(t, streamPool, batchSize, grp, x, y, zGPU)

	printLen := len(grp.GetPBytes()) / 2 // # bits / 16 for hex
	for i := uint32(0); i < batchSize; i++ {
		if zGPU.Get(i).Cmp(zCPU.Get(i)) != 0 {
			t.Errorf("Exp mismatch on index %d:\n%s\n%s",
				i, zGPU.Get(i).TextVerbose(16, printLen),
				zCPU.Get(i).TextVerbose(16, printLen))
		}
	}
	streamPool.Destroy()
}

// BenchmarkExpCPU provides a baseline with a single-threaded CPU benchmark
func runExpCPU(b *testing.B, batchSize uint32) {
	grp := initExp()

	x := grp.NewIntBuffer(batchSize, grp.NewInt(1))
	initRandomIntBuffer(grp, batchSize, x, 42)
	y := grp.NewIntBuffer(batchSize, grp.NewInt(1))
	initRandomIntBuffer(grp, batchSize, y, 43)

	z := grp.NewIntBuffer(batchSize, grp.NewInt(1))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		expCPU(b, batchSize, grp, x, y, z)
	}
}

// BenchmarkExpGPU provides a basic GPU benchmark
func runExpGPU(b *testing.B, batchSize uint32) {
	grp := initExp()

	x := grp.NewIntBuffer(batchSize, grp.NewInt(1))
	initRandomIntBuffer(grp, batchSize, x, 42)
	y := grp.NewIntBuffer(batchSize, grp.NewInt(1))
	initRandomIntBuffer(grp, batchSize, y, 43)

	z := grp.NewIntBuffer(batchSize, grp.NewInt(1))

	streamPool, err := NewStreamPool(2, 65536)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		expGPU(b, streamPool, batchSize, grp, x, y, z)
	}
	streamPool.Destroy()
}

func BenchmarkExpCPU_N(b *testing.B) {
	runExpCPU(b, uint32(b.N))
}
func BenchmarkExpCPU_1024(b *testing.B) {
	runExpCPU(b, uint32(1024))
}
func BenchmarkExpCPU_8192(b *testing.B) {
	runExpCPU(b, uint32(1024*8))
}
// Note that these tests take too long to run so are disabled
// func BenchmarkExpCPU_16384(b *testing.B) {
// 	runExpCPU(b, uint32(1024*16))
// }
// func BenchmarkExpCPU_32768(b *testing.B) {
// 	runExpCPU(b, uint32(1024*32))
// }

func BenchmarkExpGPU_N(b *testing.B) {
	runExpGPU(b, uint32(b.N))
}
func BenchmarkExpGPU_1024(b *testing.B) {
	runExpGPU(b, uint32(1024))
}
func BenchmarkExpGPU_8192(b *testing.B) {
	runExpGPU(b, uint32(1024*8))
}
func BenchmarkExpGPU_16384(b *testing.B) {
	runExpGPU(b, uint32(1024*16))
}
func BenchmarkExpGPU_32768(b *testing.B) {
	runExpGPU(b, uint32(1024*32))
}

func BenchmarkPowmCUDA4096_4096(b *testing.B) {
	g := makeTestGroup4096()
	numSlots := b.N
	input := ExpInput{
		Slots:   make([]ExpInputSlot, numSlots),
		Modulus: g.GetPBytes(),
	}
	for i := 0; i < numSlots; i++ {
		input.Slots[i] = ExpInputSlot{
			Base:     g.Random(g.NewInt(1)).Bytes(),
			Exponent: g.Random(g.NewInt(1)).Bytes(),
		}
	}

	stream, err := createStreams(1, streamSizeContaining(numSlots,
		kernelPowmOdd))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	// We'll run the exponentiation for the whole array in one chunk
	// It might be possible to run another benchmark that does two or more
	// chunks instead, which could be faster if the call could be made
	// asynchronous (which should be possible)
	resultChan := Exp(input, stream[0])
	result := <-resultChan
	if result.Err != nil {
		b.Fatal(result.Err)
	}
	b.StopTimer()
	// Write out any cached profiling data
	err = resetDevice()
	if err != nil {
		b.Fatal(err)
	}
}

// x**y, x is 2048 bits long, y is 256 bits long
func BenchmarkPowmCUDA4096_256(b *testing.B) {
	const xBitLen = 4096
	const xByteLen = xBitLen / 8
	const yBitLen = 256
	const yByteLen = yBitLen / 8
	g := makeTestGroup4096()

	numSlots := b.N
	streams, err := createStreams(1, streamSizeContaining(numSlots,
		kernelPowmOdd))
	input := ExpInput{
		Slots:   make([]ExpInputSlot, numSlots),
		Modulus: g.GetPBytes(),
	}

	for i := 0; i < numSlots; i++ {
		input.Slots[i].Exponent = g.Random(g.NewInt(1)).Bytes()[480:]
		input.Slots[i].Base = g.Random(g.NewInt(1)).Bytes()
	}
	b.ResetTimer()
	// We'll run the exponentiation for the whole array in one chunk
	// It might be possible to run another benchmark that does two or more
	// chunks instead, which could be faster if the call could be made
	// asynchronous (which should be possible)
	resultChan := Exp(input, streams[0])
	result := <-resultChan
	if result.Err != nil {
		b.Fatal(result.Err)
	}
	b.StopTimer()
	// This benchmark doesn't include converting resulting memory back to
	// cyclic ints
	// Write out any cached profiling data
	err = resetDevice()
	if err != nil {
		b.Fatal(err)
	}
}

func BenchmarkPowmCUDA4096_256_streams(b *testing.B) {
	const xBitLen = 4096
	const xByteLen = xBitLen / 8
	const yBitLen = 256
	const yByteLen = yBitLen / 8
	g := makeTestGroup4096()
	// Use two streams with 32k items per kernel launch
	numItems := 32768

	streamPool, err := NewStreamPool(2, streamSizeContaining(numItems,
		kernelPowmOdd))
	if err != nil {
		b.Fatal(err)
	}
	// Using prng because the cryptographically secure RNG used by the
	// group is too slow to feed the GPU
	rng := rand.New(rand.NewSource(5))
	b.ResetTimer()
	remainingItems := b.N
	for i := 0; i < b.N; i += numItems {
		// If part of a chunk remains, only upload that part
		remainingItems = b.N - i
		numItemsToUpload := numItems
		if remainingItems < numItems {
			numItemsToUpload = remainingItems
		}
		input := ExpInput{
			Slots:   make([]ExpInputSlot, numItemsToUpload),
			Modulus: g.GetPBytes(),
		}
		// Hopefully random number generation doesn't bottleneck things!
		for j := 0; j < numItemsToUpload; j++ {
			// Unfortunately, we can't just generate things using
			// the group, because it's too slow
			base := make([]byte, xByteLen)
			exponent := make([]byte, yByteLen)
			rng.Read(base)
			rng.Read(exponent)
			for !g.BytesInside(base, exponent) {
				rng.Read(base)
				rng.Read(exponent)
			}
			input.Slots[j] = ExpInputSlot{
				Base:     base,
				Exponent: exponent,
			}
		}
		stream := streamPool.TakeStream()
		resultChan := Exp(input, stream)
		go func() {
			result := <-resultChan
			streamPool.ReturnStream(stream)
			if result.Err != nil {
				b.Fatal(result.Err)
			}
		}()
	}
	// Empty the pool to make sure results have all been downloaded
	streamPool.TakeStream()
	streamPool.TakeStream()
	b.StopTimer()
	err = streamPool.Destroy()
	if err != nil {
		b.Fatal(err)
	}

	err = resetDevice()
	if err != nil {
		b.Fatal(err)
	}
}
