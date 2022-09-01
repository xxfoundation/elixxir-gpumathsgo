////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

//+build linux,gpu

package gpumaths

import (
	"gitlab.com/elixxir/gpumathsgo/cryptops"
	"gitlab.com/elixxir/crypto/cyclic"
	"sync"
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
// This test has a synchronization/timing issue. I need to fix it!
func TestMul2(t *testing.T) {
	//batchSize := uint32(21794)
	batchSize := uint32(217938)
	//batchSize := uint32(217940)
	grp := initMul2()

	// Generate the payload buffers
	xCPU := initRandomIntBuffer(grp, batchSize, 42, 0)
	yCPU := initRandomIntBuffer(grp, batchSize, 43, 0)

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

	//printLen := len(grp.GetPBytes()) / 2 // # bits / 16 for hex
	errCount := 0
	for i := uint32(0); i < batchSize; i++ {
		resultCPU := yCPU.Get(i)
		resultGPU := resultsGPU.Get(i)
		if resultCPU.Cmp(resultGPU) != 0 {
			errCount++
			// Try searching through CPU results - is this result elsewhere in the batch, or is it just garbage?
			//found := false
			//for j := uint32(0); j < batchSize; j++ {
			//	if resultGPU.Cmp(yCPU.Get(i)) == 0 {
			//		t.Logf("Result at index %v of GPU results found at index %v of CPU results", j, i)
			//		found = true
			//		break
			//	}
			//}
			//if !found {
			//	t.Log("Couldn't find GPU result in CPU results.")
			//}
			// Too verbose for now
			//t.Errorf("mul2 results mismatch on index %d:\n%s\n%s", i,
			//	resultCPU.TextVerbose(16, printLen),
			//	resultGPU.TextVerbose(16, printLen))
		}
	}
	t.Log("err count", errCount)
	if errCount > 0 {
		t.Errorf("Got %v differing slots in the batch", errCount)
	}
	err = streamPool.Destroy()
	if err != nil {
		t.Fatal(err)
	}
}

// single thread CPU (for simplicity) vs 1 GPU benchmark
func BenchmarkMul2CPU4096(b *testing.B) {
	grp := makeTestGroup4096()

	// mul2 4kbit*4kbit
	batchSize := uint32(b.N)
	x := initRandomIntBuffer(grp, batchSize, 42, 0)
	y := initRandomIntBuffer(grp, batchSize, 43, 0)

	b.ResetTimer()
	mul2CPU(batchSize, grp, x, y)
}

func BenchmarkMul2GPU4096(b *testing.B) {
	grp := makeTestGroup4096()

	numMuls := uint32(b.N)
	x := initRandomIntBuffer(grp, numMuls, 42, 0)
	y := initRandomIntBuffer(grp, numMuls, 43, 0)
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
				err := Mul2Chunk(streamPool, grp, xSub, ySub, resultSub)
				if err != nil {
					b.Error(err)
				}
			}
			wg.Done()
		}()
	}

	wg.Wait()
}
