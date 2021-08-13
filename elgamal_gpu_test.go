///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

//+build linux,gpu

package gpumaths

import (
	"git.xx.network/elixxir/crypto/cryptops"
	"git.xx.network/elixxir/crypto/cyclic"
	"testing"
)

// Helper functions shared by tests are located in gpu_test.go

func initElGamal(batchSize uint32) (*cyclic.Group, *cyclic.Int,
	*cyclic.IntBuffer, *cyclic.IntBuffer) {
	grp := initTestGroup()

	// Set up Keys and public cypher key for operation
	PublicCypherKey := grp.NewInt(1)
	grp.Random(PublicCypherKey)

	// Set the R and Y_R or other Keys (phase/share)
	phaseKeys := grp.NewIntBuffer(batchSize, grp.NewInt(1))
	shareKeys := grp.NewIntBuffer(batchSize, grp.NewInt(1))
	initKeys(grp, batchSize, phaseKeys, shareKeys, int64(42))
	return grp, PublicCypherKey, phaseKeys, shareKeys
}

func elgamalCPU(batchSize uint32, grp *cyclic.Group, PublicCypherKey *cyclic.Int, phaseKeys, shareKeys, KeysPayload, CypherPayload *cyclic.IntBuffer) {
	for i := uint32(0); i < batchSize; i++ {
		cryptops.ElGamal(grp,
			phaseKeys.Get(i), shareKeys.Get(i),
			PublicCypherKey,
			KeysPayload.Get(i), CypherPayload.Get(i))
	}
}

func elgamalGPU(t testing.TB, streamPool *StreamPool,
	grp *cyclic.Group, PublicCypherKey *cyclic.Int, phaseKeys, shareKeys,
	KeysPayload, CypherPayload *cyclic.IntBuffer) {
	err := ElGamalChunk(streamPool, grp, phaseKeys, shareKeys,
		PublicCypherKey, KeysPayload, CypherPayload)
	if err != nil {
		t.Fatal(err)
	}
}

// Runs precomp decrypt test with GPU stream pool and graphs
func TestElGamal(t *testing.T) {
	batchSize := uint32(1024)
	grp, PublicCypherKey, phaseKeys, shareKeys := initElGamal(batchSize)

	// Generate the payload buffers
	KeysPayloadCPU := initRandomIntBuffer(grp, batchSize, 42, 256/8)
	CypherPayloadCPU := initRandomIntBuffer(grp, batchSize, 43, grp.GetP().BitLen())

	// Make a copy for GPU Processing
	KeysPayloadGPU := KeysPayloadCPU.DeepCopy()
	CypherPayloadGPU := CypherPayloadCPU.DeepCopy()

	// Run CPU
	elgamalCPU(batchSize, grp, PublicCypherKey, phaseKeys, shareKeys, KeysPayloadCPU, CypherPayloadCPU)

	// Run GPU
	streamPool, err := NewStreamPool(2, 65536)
	if err != nil {
		t.Fatal(err)
	}
	elgamalGPU(t, streamPool, grp, PublicCypherKey,
		phaseKeys, shareKeys, KeysPayloadGPU, CypherPayloadGPU)

	printLen := len(grp.GetPBytes()) / 2 // # bits / 16 for hex
	for i := uint32(0); i < batchSize; i++ {
		KPGPU := KeysPayloadGPU.Get(i)
		KPCPU := KeysPayloadCPU.Get(i)
		if KPGPU.Cmp(KPCPU) != 0 {
			t.Errorf("KeysPayloadMisMatch on index %d:\n%s\n%s", i,
				KPGPU.TextVerbose(16, printLen),
				KPCPU.TextVerbose(16, printLen))
		}

		CPGPU := CypherPayloadGPU.Get(i)
		CPCPU := CypherPayloadCPU.Get(i)
		if CPGPU.Cmp(CPCPU) != 0 {
			t.Errorf("CypherPayload mismatch on index %d:\n%s\n%s",
				i, CPGPU.TextVerbose(16, printLen),
				CPCPU.TextVerbose(16, printLen))
		}
	}
	err = streamPool.Destroy()
	if err != nil {
		t.Error(err)
	}
}

// BenchmarkElGamalCPU provides a baseline with a single-threaded CPU benchmark
func runElGamalCPU(b *testing.B, batchSize uint32) {
	grp, PublicCypherKey, phaseKeys, shareKeys := initElGamal(batchSize)

	// Generate the payload buffers
	KeysPayloadCPU := initRandomIntBuffer(grp, batchSize, 42, 0)
	CypherPayloadCPU := initRandomIntBuffer(grp, batchSize, 43, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		elgamalCPU(batchSize, grp, PublicCypherKey, phaseKeys, shareKeys, KeysPayloadCPU, CypherPayloadCPU)
	}
}

// BenchmarkElGamalGPU provides a basic GPU benchmark
func runElGamalGPU(b *testing.B, batchSize uint32) {
	grp, PublicCypherKey, phaseKeys, shareKeys := initElGamal(batchSize)

	// Generate the payload buffers
	KeysPayloadGPU := initRandomIntBuffer(grp, batchSize, 42, 0)
	CypherPayloadGPU := initRandomIntBuffer(grp, batchSize, 43, 0)

	streamPool, err := NewStreamPool(2, 65536)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		elgamalGPU(b, streamPool, grp, PublicCypherKey,
			phaseKeys, shareKeys, KeysPayloadGPU, CypherPayloadGPU)
	}
	err = streamPool.Destroy()
	if err != nil {
		b.Error(err)
	}
}

func BenchmarkElGamalCPU_N(b *testing.B) {
	runElGamalCPU(b, uint32(b.N))
}
func BenchmarkElGamalCPU_1024(b *testing.B) {
	runElGamalCPU(b, uint32(1024))
}
func BenchmarkElGamalCPU_8192(b *testing.B) {
	runElGamalCPU(b, uint32(1024*8))
}
func BenchmarkElGamalCPU_16384(b *testing.B) {
	runElGamalCPU(b, uint32(1024*16))
}
func BenchmarkElGamalCPU_32768(b *testing.B) {
	runElGamalCPU(b, uint32(1024*32))
}

func BenchmarkElGamalGPU_N(b *testing.B) {
	runElGamalGPU(b, uint32(b.N))
}
func BenchmarkElGamalGPU_1024(b *testing.B) {
	runElGamalGPU(b, uint32(1024))
}
func BenchmarkElGamalGPU_8192(b *testing.B) {
	runElGamalGPU(b, uint32(1024*8))
}
func BenchmarkElGamalGPU_16384(b *testing.B) {
	runElGamalGPU(b, uint32(1024*16))
}
func BenchmarkElGamalGPU_32768(b *testing.B) {
	runElGamalGPU(b, uint32(1024*32))
}

// BenchmarkElGamalCUDA4096_256_streams benchmarks the ElGamal and stream functions directly.
func BenchmarkElGamalCUDA4096_256_streams(b *testing.B) {
	const xBitLen = 4096
	const xByteLen = xBitLen / 8
	const yBitLen = 4096
	const yByteLen = yBitLen / 8
	g := makeTestGroup4096()
	env := &gpumathsEnv4096
	// Use two streams with 32k items per kernel launch
	numItems := 32768

	// OK, this shouldn't cause the test to run forever if the stream size is smaller than it should be (like this)
	// In real-world usage, the number of slots passed in should be determined by what the stream supports
	//  (i.e. check stream.MaxSlotsElgamal)
	streamPool, err := NewStreamPool(2, env.streamSizeContaining(numItems, kernelElgamal))
	if err != nil {
		b.Fatal(err)
	}
	// Using prng because the cryptographically secure RNG used by the group is too slow to feed the GPU
	b.ResetTimer()
	remainingItems := b.N
	for i := 0; i < b.N; i += numItems {
		// If part of a chunk remains, only upload that part
		remainingItems = b.N - i
		numItemsToUpload := numItems
		if remainingItems < numItems {
			numItemsToUpload = remainingItems
		}
		PublicCypherKey := g.Random(g.NewInt(1))
		// Hopefully random number generation doesn't bottleneck things!
		key := initRandomIntBuffer(g, uint32(numItemsToUpload), 42, xByteLen)
		ecrKey := initRandomIntBuffer(g, uint32(numItemsToUpload), 43, xByteLen)
		cypher := initRandomIntBuffer(g, uint32(numItemsToUpload), 44, xByteLen)
		privateKey := initRandomIntBuffer(g, uint32(numItemsToUpload), 45, yByteLen)
		stream := streamPool.TakeStream()
		resultChan := elGamal(g, key, privateKey, PublicCypherKey, ecrKey, cypher, env, stream)
		go func() {
			err := <-resultChan
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
