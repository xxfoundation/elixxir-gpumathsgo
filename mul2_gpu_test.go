////////////////////////////////////////////////////////////////////////////////
// Copyright © 2019 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

//+build linux,gpu

package gpumaths

import (
	"gitlab.com/elixxir/crypto/cryptops"
	"gitlab.com/elixxir/crypto/cyclic"
	"testing"
)

// Helper functions shared by tests are located in gpu_test.go

func initMul2() *cyclic.Group {
	return initTestGroup()
}

func mul2CPU(batchSize uint32, grp *cyclic.Group,
	x, y *cyclic.IntBuffer) {
	for i := uint32(0); i < batchSize; i++ {
		cryptops.Mul2(grp, x.Get(i), y.Get(i))
	}
}

func mul2GPU(t testing.TB, streamPool *StreamPool,
	grp *cyclic.Group, x, y, result *cyclic.IntBuffer) {
	err := Mul2Chunk(streamPool, grp, x, y, result)
	if err != nil {
		t.Fatal(err)
	}
}

// Runs precomp decrypt test with GPU stream pool and graphs
func TestMul2(t *testing.T) {
	batchSize := uint32(1024)
	grp := initMul2()

	// Generate the payload buffers
	xCPU := grp.NewIntBuffer(batchSize, grp.NewInt(1))
	initRandomIntBuffer(grp, batchSize, xCPU, 42)
	yCPU := grp.NewIntBuffer(batchSize, grp.NewInt(1))
	initRandomIntBuffer(grp, batchSize, yCPU, 43)

	// Make a copy for GPU Processing
	xGPU := xCPU.DeepCopy()
	yGPU := yCPU.DeepCopy()
	resultsGPU := grp.NewIntBuffer(batchSize, grp.NewInt(1))

	// Run CPU. Results are in yCPU
	mul2CPU(batchSize, grp, xCPU, yCPU)

	// Run GPU
	streamPool, err := NewStreamPool(2, 65536)
	if err != nil {
		t.Fatal(err)
	}
	mul2GPU(t, streamPool, grp, xGPU, yGPU, resultsGPU)

	printLen := len(grp.GetPBytes()) / 2 // # bits / 16 for hex
	for i := uint32(0); i < batchSize; i++ {
		resultCPU := yCPU.Get(i)
		resultGPU := resultsGPU.Get(i)
		if resultCPU.Cmp(resultGPU) != 0 {
			t.Errorf("mul2 results mismatch on index %d:\n%s\n%s", i,
				resultCPU.TextVerbose(16, printLen),
				resultGPU.TextVerbose(16, printLen))
		}
	}
	streamPool.Destroy()
}
