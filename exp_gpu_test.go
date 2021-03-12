///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

//+build linux,gpu

package gpumaths

import (
	"gitlab.com/elixxir/crypto/cryptops"
	"gitlab.com/elixxir/crypto/cyclic"
	"testing"
)

// Helper functions shared by tests are located in gpu_test.go

func initExp() *cyclic.Group {
	return initTestGroup()
}

func expCPU(batchSize uint32, grp *cyclic.Group, x, y, z *cyclic.IntBuffer) {
	for i := uint32(0); i < batchSize; i++ {
		cryptops.Exp(grp, x.Get(i), y.Get(i), z.Get(i))
	}
}

func expGPU(t testing.TB, streamPool *StreamPool, grp *cyclic.Group, x, y, z *cyclic.IntBuffer) {
	_, err := ExpChunk(streamPool, grp, x, y, z)
	if err != nil {
		t.Fatal(err)
	}
}

// Runs exponentiation test with GPU stream pool and graphs
func TestExp(t *testing.T) {
	batchSize := uint32(1024)
	grp := initExp()

	x := initRandomIntBuffer(grp, batchSize, 42, 0)
	y := initRandomIntBuffer(grp, batchSize, 43, 0)

	zCPU := grp.NewIntBuffer(batchSize, grp.NewInt(1))
	zGPU := grp.NewIntBuffer(batchSize, grp.NewInt(1))

	// Run CPU
	expCPU(batchSize, grp, x, y, zCPU)

	// Run GPU
	streamPool, err := NewStreamPool(2, 65536)
	if err != nil {
		t.Fatal(err)
	}
	expGPU(t, streamPool, grp, x, y, zGPU)

	printLen := len(grp.GetPBytes()) / 2 // # bits / 16 for hex
	for i := uint32(0); i < batchSize; i++ {
		if zGPU.Get(i).Cmp(zCPU.Get(i)) != 0 {
			t.Errorf("exp mismatch on index %d:\n%s\n%s",
				i, zGPU.Get(i).TextVerbose(16, printLen),
				zCPU.Get(i).TextVerbose(16, printLen))
		}
	}
	err = streamPool.Destroy()
	if err != nil {
		t.Error(err)
	}
}

// BenchmarkExpCPU provides a baseline with a single-threaded CPU benchmark
func runExpCPU(b *testing.B, batchSize uint32) {
	grp := initExp()

	x := initRandomIntBuffer(grp, batchSize, 42, 0)
	y := initRandomIntBuffer(grp, batchSize, 43, 0)

	z := grp.NewIntBuffer(batchSize, grp.NewInt(1))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		expCPU(batchSize, grp, x, y, z)
	}
}

// BenchmarkExpGPU provides a basic GPU benchmark
func runExpGPU(b *testing.B, batchSize uint32) {
	grp := initExp()

	x := initRandomIntBuffer(grp, batchSize, 42, 0)
	y := initRandomIntBuffer(grp, batchSize, 43, 0)

	z := grp.NewIntBuffer(batchSize, grp.NewInt(1))

	streamPool, err := NewStreamPool(2, 65536)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		expGPU(b, streamPool, grp, x, y, z)
	}
	err = streamPool.Destroy()
	if err != nil {
		b.Error(err)
	}
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
	env := &gpumathsEnv4096
	numSlots := uint32(b.N)
	Base := initRandomIntBuffer(g, numSlots, 42, 0)
	Exponent := initRandomIntBuffer(g, numSlots, 42, 0)
	Results := g.NewIntBuffer(numSlots, g.NewInt(1))

	stream, err := createStreams(1, env.streamSizeContaining(int(numSlots),
		kernelPowmOdd))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	// We'll run the exponentiation for the whole array in one chunk
	// It might be possible to run another benchmark that does two or more
	// chunks instead, which could be faster if the call could be made
	// asynchronous (which should be possible)
	errors := exp(g, Base, Exponent, Results, env, stream[0])
	err = <-errors
	if err != nil {
		b.Fatal(err)
	}
	b.StopTimer()
	// Write out any cached profiling data
	//err = resetDevice()
	//if err != nil {
	//	b.Fatal(err)
	//}
}

// x**y, x is 2048 bits long, y is 256 bits long
func BenchmarkPowmCUDA4096_256(b *testing.B) {

	g := makeTestGroup4096()
	env := &gpumathsEnv4096

	numSlots := uint32(b.N)
	streams, err := createStreams(1, env.streamSizeContaining(int(numSlots),
		kernelPowmOdd))

	Base := initRandomIntBuffer(g, numSlots, 42, 0)
	Exponent := initRandomIntBuffer(g, numSlots, 42, 256/8)
	Results := g.NewIntBuffer(numSlots, g.NewInt(1))
	b.ResetTimer()
	// We'll run the exponentiation for the whole array in one chunk
	// It might be possible to run another benchmark that does two or more
	// chunks instead, which could be faster if the call could be made
	// asynchronous (which should be possible)
	resultChan := exp(g, Base, Exponent, Results, env, streams[0])
	err = <-resultChan
	if err != nil {
		b.Fatal(err)
	}
	b.StopTimer()
	// This benchmark doesn't include converting resulting memory back to
	// cyclic ints
	// Write out any cached profiling data
	//err = resetDevice()
	//if err != nil {
	//	b.Fatal(err)
	//}
}

func BenchmarkPowmCUDA2048_256_streams(b *testing.B) {

	const yBitLen = 256
	const yByteLen = yBitLen / 8
	g := makeTestGroup2048()
	env := &gpumathsEnv2048
	// Use two streams with 32k items per kernel launch
	numItems := 32768

	streamPool, err := NewStreamPool(2, env.streamSizeContaining(numItems,
		kernelPowmOdd))
	if err != nil {
		b.Fatal(err)
	}
	// Using prng because the cryptographically secure RNG used by the
	// group is too slow to feed the GPU
	b.ResetTimer()
	remainingItems := b.N
	for i := 0; i < b.N; i += numItems {
		// If part of a chunk remains, only upload that part
		remainingItems = b.N - i
		numItemsToUpload := numItems
		if remainingItems < numItems {
			numItemsToUpload = remainingItems
		}
		// Hopefully random number generation doesn't bottleneck things!
		base := initRandomIntBuffer(g, uint32(numItemsToUpload), 42, 0)
		exponent := initRandomIntBuffer(g, uint32(numItemsToUpload), 42, yByteLen)
		results := g.NewIntBuffer(uint32(numItemsToUpload), g.NewInt(1))
		stream := streamPool.TakeStream()
		errChan := exp(g, base, exponent, results, env, stream)
		go func() {
			err := <-errChan
			streamPool.ReturnStream(stream)
			if err != nil {
				b.Fatal(err)
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

	//err = resetDevice()
	//if err != nil {
	//	b.Fatal(err)
	//}
}

func BenchmarkPowmCUDA4096_256_streams(b *testing.B) {

	const yBitLen = 256
	const yByteLen = yBitLen / 8
	g := makeTestGroup4096()
	env := &gpumaths4096{}
	// Use two streams with 32k items per kernel launch
	numItems := 32768

	streamPool, err := NewStreamPool(2, env.streamSizeContaining(numItems,
		kernelPowmOdd))
	if err != nil {
		b.Fatal(err)
	}
	// Using prng because the cryptographically secure RNG used by the
	// group is too slow to feed the GPU
	b.ResetTimer()
	remainingItems := b.N
	for i := 0; i < b.N; i += numItems {
		// If part of a chunk remains, only upload that part
		remainingItems = b.N - i
		numItemsToUpload := numItems
		if remainingItems < numItems {
			numItemsToUpload = remainingItems
		}
		// Hopefully random number generation doesn't bottleneck things!
		base := initRandomIntBuffer(g, uint32(numItemsToUpload), 42, 0)
		exponent := initRandomIntBuffer(g, uint32(numItemsToUpload), 42, yByteLen)
		results := g.NewIntBuffer(uint32(numItemsToUpload), g.NewInt(1))
		stream := streamPool.TakeStream()
		errChan := exp(g, base, exponent, results, env, stream)
		go func() {
			err := <-errChan
			streamPool.ReturnStream(stream)
			if err != nil {
				b.Fatal(err)
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

	//err = resetDevice()
	//if err != nil {
	//	b.Fatal(err)
	//}
}
