///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

//+build linux,gpu

package gpumaths

/*#cgo LDFLAGS: -Llib -lpowmosm75 -Wl,-rpath -Wl,./lib:/opt/xxnetwork/lib
  #cgo CFLAGS: -I./cgbnBindings/powm -I/opt/xxnetwork/include
  #include <powm_odd_export.h>
*/
import "C"
import (
	"gitlab.com/elixxir/crypto/cyclic"
	"gitlab.com/xx_network/crypto/large"
	"math/rand"
	"time"
)

// mul2_gpu.go contains the CUDA ops for the Mul2 operation. Mul2(...)
// performs the actual call into the library and Mul2Chunk implements
// the streaming interface function called by the server implementation.

const kernelMul2 = C.KERNEL_MUL2

// Mul2Chunk performs the Mul2 operation on the cypher and precomputation
// payloads
// Precondition: All int buffers must have the same length
var Mul2Chunk Mul2ChunkPrototype = func(p *StreamPool, g *cyclic.Group,
	x *cyclic.IntBuffer, y *cyclic.IntBuffer, results *cyclic.IntBuffer) error {
	// Populate Mul2 inputs
	numSlots := uint32(x.Len())

	// Run kernel on the inputs
	stream := p.TakeStream()
	defer p.ReturnStream(stream)
	env := chooseEnv(g)
	maxSlotsMul2 := uint32(env.maxSlots(len(stream.cpuData), kernelMul2))
	for i := uint32(0); i < numSlots; i += maxSlotsMul2 {
		sliceEnd := i
		// Don't slice beyond the end of the input slice
		if i+maxSlotsMul2 <= numSlots {
			sliceEnd += maxSlotsMul2
		} else {
			sliceEnd = numSlots
		}
		err := <-Mul2(g, x.GetSubBuffer(i, sliceEnd), y.GetSubBuffer(i, sliceEnd), results.GetSubBuffer(i, sliceEnd), env, stream)
		if err != nil {
			return err
		}
	}

	return nil
}

// Mul2 runs the mul2 operation on precomputation and cypher payloads inside
// the GPU
// NOTE: publicCypherKey and prime should be byte slices obtained by running
//       .Bytes() on the large int
// The resulting byte slice should be trimmed and should be less than or
// equal to the length of the template-instantiated BN on the GPU.
// bnLength is a length in bits
// TODO validate BN length in code (i.e. pick kernel variants based on bn
// length)
// puts output in results int buffer
func Mul2(g *cyclic.Group, x *cyclic.IntBuffer, y *cyclic.IntBuffer, results *cyclic.IntBuffer, env gpumathsEnv, stream Stream) chan error {
	debugPrint := false
	callId := rand.Intn(9999)
	start := time.Now()
	//if debugPrint {
	//	println("starting mul2 call", callId, "with", len(input.Slots), "slots at", start.String())
	//}
	start = time.Now()
	//Return the result later, when the GPU job finishes
	resultChan := make(chan error, 1)
	go func() {
		// Arrange memory into stream buffers
		numSlots := uint32(x.Len())

		// TODO clean this up by implementing the
		// arrangement/dearrangement with reader/writer interfaces
		// or smth
		constants := stream.getCpuConstantsWords(env, kernelMul2)
		// TODO right pad with zeroes if there isn't enough space
		copy(constants, g.GetP().Bits())
		//fmt.Printf("%x %v\n", constants, len(constants))
		offset := 0
		// Prime
		bnLengthWords := env.getWordLen()
		//panic(bnLengthWords)
		//getSliceStart := time.Now()
		//println("time for getting consts slice:",callId, time.Since(getSliceStart))
		offset += bnLengthWords

		inputs := stream.getCpuInputsWords(env, kernelMul2, int(numSlots))
		offset = 0
		for i := uint32(0); i < numSlots; i++ {
			// Put the first operand for this slot
			// TODO rightpad with zeroes
			copy(inputs[offset:offset+bnLengthWords], x.Get(i).Bits())
			offset += bnLengthWords
			// Put the second operand for this slot
			copy(inputs[offset:offset+bnLengthWords], y.Get(i).Bits())
			offset += bnLengthWords
		}
		if debugPrint {
			println("Call", callId, "post input arrangement", time.Since(start))
			start = time.Now()
		}

		// Upload, run, wait for download
		err := env.enqueue(stream, kernelMul2, int(numSlots))
		// Enqueue could memset outputs to zero before returning
		if debugPrint {
			println("Call", callId, "post put", time.Since(start))
			start = time.Now()
		}
		if err != nil {
			resultChan <- err
			return
		}
		//err = env.run(stream)
		//println("Call", callId, "post run", time.Since(start))
		//start = time.Now()
		//if err != nil {
		//	resultChan <- Mul2Result{Err: err}
		//	return
		//}
		//err = env.download(stream)
		//println("Call", callId, "post download", time.Since(start))
		//start = time.Now()
		//if err != nil {
		//	resultChan <- Mul2Result{Err: err}
		//	return
		//}

		outputs := stream.getCpuOutputsWords(env, kernelMul2, int(numSlots))

		// Wait on things to finish with Cuda
		// Blocks for a long time? Could be a problem?
		// Attempt 1: basic polling loop
		//for getErr := get2(stream); getErr != nil; getErr = get2(stream) {
		//	time.Sleep(200*time.Microsecond)
		//}
		// TODO estimate sleep fraction based on variance?
		// probably only do that if this shows perf improvement...
		//time.Sleep(time.Duration(float32(stream.elapsedTimeMovingAverage) * 25.5))
		// Get elapsed time that the kernel ran
		//var elapsedTime time.Duration
		//elapsedTime, err = get(stream)
		//if stream.elapsedTimeMovingAverage != 0 {
		//	Fold elapsed time into moving average
		//stream.elapsedTimeMovingAverage = stream.elapsedTimeMovingAverage*3/4 + elapsedTime/4
		//} else {
		//	stream.elapsedTimeMovingAverage = elapsedTime
		//}

		_, err = get(stream)

		// NOTE this did not work
		// Trying a different strategy...
		// Instead of get(), let's simply read the memory until certain areas are nonzero!
		// This DOES NOT guarantee all results have been copied to the buffer
		// just that some of them may exist there
		//loop:
		//for {
		//	for i := range results {
		//		if results[i] != 0 {
		//			We have some results!!!
		//break loop
		//}
		//}
		//Good to sleep?
		//time.Sleep(100*time.Nanosecond)
		//}
		//

		if debugPrint {
			println("Call", callId, "post get", time.Since(start))
			start = time.Now()
		}
		if err != nil {
			resultChan <- err
			return
		}

		// Everything is OK, so let's go ahead and import the results

		offset = 0
		for i := uint32(0); i < numSlots; i++ {
			// Output the computed result into each slot
			thisOutput := outputs[offset : offset+bnLengthWords]
			thisOutputCopy := make(large.Bits, len(thisOutput))
			copy(thisOutputCopy, thisOutput)
			g.SetBits(results.Get(i), thisOutputCopy)
			offset += bnLengthWords
		}

		if debugPrint {
			println("Call", callId, "post output arrangement", time.Since(start))
		}

		resultChan <- nil
	}()
	return resultChan
}
