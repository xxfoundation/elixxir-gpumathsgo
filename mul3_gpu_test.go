//+build linux,gpu

package gpumaths

import (
	"gitlab.com/elixxir/crypto/cryptops"
	"gitlab.com/elixxir/crypto/cyclic"
	"testing"
)

func TestMul3Chunk(t *testing.T) {
	batchSize := uint32(24601)
	grp := initTestGroup()

	// Generate the payload buffers
	xCPU := initRandomIntBuffer(grp, batchSize, 42, 0)
	yCPU := initRandomIntBuffer(grp, batchSize, 43, 0)
	zCPU := initRandomIntBuffer(grp, batchSize, 44, 0)

	// Make a copy for GPU Processing
	xGPU := xCPU.DeepCopy()
	yGPU := yCPU.DeepCopy()
	zGPU := zCPU.DeepCopy()
	resultsGPU := grp.NewIntBuffer(batchSize, grp.NewInt(1))

	// Run CPU. Results are in zCPU
	mul3CPU(batchSize, grp, xCPU, yCPU, zCPU)

	// Run GPU
	streamPool, err := NewStreamPool(2, 65536)
	if err != nil {
		t.Fatal(err)
	}
	mul3GPU(t, streamPool, grp, xGPU, yGPU, zGPU, resultsGPU)

	printLen := len(grp.GetPBytes()) / 2 // # bits / 16 for hex
	for i := uint32(0); i < batchSize; i++ {
		resultCPU := zCPU.Get(i)
		resultGPU := resultsGPU.Get(i)
		if resultCPU.Cmp(resultGPU) != 0 {
			t.Errorf("mul3 results mismatch on index %d:\n%s\n%s", i,
				resultCPU.TextVerbose(16, printLen),
				resultGPU.TextVerbose(16, printLen))
		}
	}
	err = streamPool.Destroy()
	if err != nil {
		t.Fatal(err)
	}
}

func mul3GPU(t *testing.T, pool *StreamPool, grp *cyclic.Group, xGPU *cyclic.IntBuffer, yGPU *cyclic.IntBuffer, zGPU *cyclic.IntBuffer, resultsGPU *cyclic.IntBuffer) {
	err := Mul3Chunk(pool, grp, xGPU, yGPU, zGPU, resultsGPU)
	if err != nil {
		t.Error(err)
	}
}

func mul3CPU(size uint32, grp *cyclic.Group, xCPU *cyclic.IntBuffer, yCPU *cyclic.IntBuffer, zCPU *cyclic.IntBuffer) {
	for i := uint32(0); i < size; i++ {
		cryptops.Mul3(grp, xCPU.Get(i), yCPU.Get(i), zCPU.Get(i))
	}
}
