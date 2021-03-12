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

func initReveal() (*cyclic.Group, *cyclic.Int) {
	grp := initTestGroup()

	// Set up Keys and public cypher key for operation
	PublicCypherKey := grp.NewInt(1)
	grp.FindSmallCoprimeInverse(PublicCypherKey, 256)

	return grp, PublicCypherKey
}

func revealCPU(batchSize uint32, grp *cyclic.Group,
	PublicCypherKey *cyclic.Int, CypherPayload *cyclic.IntBuffer) {
	for i := uint32(0); i < batchSize; i++ {
		cryptops.RootCoprime(grp,
			CypherPayload.Get(i),
			PublicCypherKey,
			grp.NewInt(1))
	}
}

func revealGPU(t testing.TB, streamPool *StreamPool,
	grp *cyclic.Group, PublicCypherKey *cyclic.Int,
	CypherPayload *cyclic.IntBuffer) {

	err := RevealChunk(streamPool, grp, PublicCypherKey, CypherPayload, CypherPayload)
	if err != nil {
		t.Fatal(err)
	}
}

func runRevealCPU(b *testing.B, batchSize uint32) {
	grp, publicCypherKey := initReveal()

	// Generate the cypher text buffer
	cypherPayload := initRandomIntBuffer(grp, batchSize, 11, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		revealCPU(batchSize, grp, publicCypherKey, cypherPayload)
	}
}

func runRevealGPU(b *testing.B, batchSize uint32) {
	grp, publicCypherKey := initReveal()

	// Generate the cypher text buffer
	cypherPayload := initRandomIntBuffer(grp, batchSize, 11, 0)

	env := chooseEnv(grp)
	memSize := env.streamSizeContaining(int(batchSize), kernelReveal)
	b.Log(batchSize, memSize)
	streamPool, err := NewStreamPool(2, memSize)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		revealGPU(b, streamPool, grp, publicCypherKey, cypherPayload)
	}
}

func BenchmarkRevealCPU_N(b *testing.B) {
	runRevealCPU(b, uint32(b.N))
}
func BenchmarkRevealCPU_1024(b *testing.B) {
	runRevealCPU(b, uint32(1024))
}
func BenchmarkRevealCPU_8192(b *testing.B) {
	runRevealCPU(b, uint32(1024*8))
}

func BenchmarkRevealGPU_N(b *testing.B) {
	runRevealGPU(b, uint32(b.N))
}
func BenchmarkRevealGPU_1024(b *testing.B) {
	runRevealGPU(b, uint32(1024))
}
func BenchmarkRevealGPU_8192(b *testing.B) {
	runRevealGPU(b, uint32(1024*8))
}
func BenchmarkRevealGPU_16384(b *testing.B) {
	runRevealGPU(b, uint32(1024*16))
}
func BenchmarkRevealGPU_32768(b *testing.B) {
	runRevealGPU(b, uint32(1024*32))
}

func TestReveal(t *testing.T) {
	g, publicCypherKey := initReveal()
	n := uint32(128)
	cypherCPU := initRandomIntBuffer(g, n, 42, 0)
	cypherGPU := cypherCPU.DeepCopy()

	streamPool, err := NewStreamPool(2, 65536)
	if err != nil {
		t.Fatal(err)
	}
	err = RevealChunk(streamPool, g, publicCypherKey, cypherGPU, cypherGPU)
	if err != nil {
		t.Error(err)
	}

	for i := 0; i < cypherCPU.Len(); i++ {
		// Is this the correct invocation?
		cryptops.RootCoprime(g, cypherCPU.Get(uint32(i)), publicCypherKey, cypherCPU.Get(uint32(i)))
	}

	// Compare results
	for i := uint32(0); i < uint32(cypherCPU.Len()); i++ {
		cpuResult := cypherCPU.Get(i)
		gpuResult := cypherGPU.Get(i)
		if cpuResult.Cmp(gpuResult) != 0 {
			t.Errorf("results differed at index %v", i)
		}
	}
}
