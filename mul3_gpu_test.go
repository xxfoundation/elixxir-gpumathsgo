//+build linux,gpu

package gpumaths

import (
	"gitlab.com/elixxir/crypto/cyclic"
	"gitlab.com/elixxir/gpumathsgo/cryptops"
	"sync"
	"testing"
)

func BenchmarkMul3GPU4096(b *testing.B) {
	grp := makeTestGroup4096()

	numMuls := uint32(b.N)
	x := initRandomIntBuffer(grp, numMuls, 42, 0)
	y := initRandomIntBuffer(grp, numMuls, 43, 0)
	z := initRandomIntBuffer(grp, numMuls, 44, 0)
	result := grp.NewIntBuffer(numMuls, grp.NewInt(1))

	numStreams := 2
	streamPool, err := NewStreamPool(numStreams, 6553600)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	// Set size of gpu jobs
	const gpuJobSize = 64
	chunkStart := uint32(0)
	var chunkStartLock sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < numStreams; i++ {
		wg.Add(1)
		// We'll use both streams, as the server does
		go func() {
			for {
				chunkStartLock.Lock()
				thisChunkStart := chunkStart
				chunkStart = thisChunkStart + gpuJobSize
				chunkStartLock.Unlock()
				if thisChunkStart >= numMuls {
					break
				}
				thisChunkEnd := thisChunkStart + gpuJobSize
				if thisChunkEnd > numMuls {
					// bound subbuffer end to subbuffer end size
					thisChunkEnd = numMuls
				}
				resultSub := result.GetSubBuffer(thisChunkStart, thisChunkEnd)
				xSub := x.GetSubBuffer(thisChunkStart, thisChunkEnd)
				ySub := y.GetSubBuffer(thisChunkStart, thisChunkEnd)
				zSub := z.GetSubBuffer(thisChunkStart, thisChunkEnd)
				err := Mul3Chunk(streamPool, grp, xSub, ySub, zSub, resultSub)
				if err != nil {
					b.Error(err)
				}
			}
			wg.Done()
		}()
	}

	wg.Wait()

}

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
