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
	"fmt"
	"gitlab.com/elixxir/crypto/cyclic"
	"log"
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
	numSlots := x.Len()
	input := Mul2Input{
		Slots: make([]Mul2InputSlot, numSlots),
		Prime: g.GetPBytes(),
	}
	for i := uint32(0); i < uint32(numSlots); i++ {
		input.Slots[i] = Mul2InputSlot{
			X: x.Get(i).Bytes(),
			Y: y.Get(i).Bytes(),
		}
	}

	// Run kernel on the inputs
	stream := p.TakeStream()
	defer p.ReturnStream(stream)
	env := chooseEnv(g)
	maxSlotsMul2 := env.maxSlots(len(stream.cpuData), kernelMul2)
	for i := 0; i < numSlots; i += maxSlotsMul2 {
		sliceEnd := i
		// Don't slice beyond the end of the input slice
		if i+maxSlotsMul2 <= numSlots {
			sliceEnd += maxSlotsMul2
		} else {
			sliceEnd = numSlots
		}
		thisInput := Mul2Input{
			Slots: input.Slots[i:sliceEnd],
			Prime: input.Prime,
		}
		result := <-Mul2(thisInput, env, stream)
		if result.Err != nil {
			return result.Err
		}
		// Populate with results
		for j := range result.Slots {
			g.SetBytes(results.Get(uint32(i+j)),
				result.Slots[j].Result)
		}
	}

	return nil
}

// Bounds check to make sure that the stream can take all the inputs
func validateMul2Input(input Mul2Input, env gpumathsEnv, stream Stream) {
	maxSlotsMul2 := env.maxSlots(len(stream.cpuData), kernelMul2)
	if len(input.Slots) > maxSlotsMul2 {
		// This can only happen because of user error (unlike Cuda
		// problems), so panic to make the error apparent
		log.Panicf(fmt.Sprintf("%v slots is more than this stream's "+
			"max of %v for Mul2 kernel",
			len(input.Slots), maxSlotsMul2))
	}
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
func Mul2(input Mul2Input, env gpumathsEnv, stream Stream) chan Mul2Result {
	//callId := rand.Intn(9999)
	//start := time.Now()
	//println("starting mul2 call", callId, "with", len(input.Slots), "slots at", start.String())
	// Return the result later, when the GPU job finishes
	resultChan := make(chan Mul2Result, 1)
	go func() {
		// Validation took a long time too?
		// It's a cgo call, but we could cache it like before...
		validateMul2Input(input, env, stream)
		//println("Call", callId, "post validation", time.Since(start))
		//start = time.Now()

		// Arrange memory into stream buffers
		numSlots := len(input.Slots)

		// TODO clean this up by implementing the
		// arrangement/dearrangement with reader/writer interfaces
		// or smth
		constants := stream.getCpuConstants(env, kernelMul2)
		offset := 0
		// Prime
		bnLengthBytes := env.getByteLen()
		//getSliceStart := time.Now()
		putInt(constants[offset:offset+bnLengthBytes],
			input.Prime, bnLengthBytes)
		//println("time for getting consts slice:",callId, time.Since(getSliceStart))
		offset += bnLengthBytes

		inputs := stream.getCpuInputs(env, kernelMul2, numSlots)
		offset = 0
		for i := 0; i < numSlots; i++ {
			// Put the first operand for this slot
			putInt(inputs[offset:offset+bnLengthBytes],
				input.Slots[i].X, bnLengthBytes)
			offset += bnLengthBytes
			// Put the second operand for this slot
			putInt(inputs[offset:offset+bnLengthBytes],
				input.Slots[i].Y, bnLengthBytes)
			offset += bnLengthBytes
		}
		//println("Call", callId, "post arrangement", time.Since(start))
		//start = time.Now()

		// Upload, run, wait for download
		err := env.enqueue(stream, kernelMul2, numSlots)
		//println("Call", callId, "post put", time.Since(start))
		//start = time.Now()
		if err != nil {
			resultChan <- Mul2Result{Err: err}
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

		// Results will be stored in this buffer
		resultBuf := make([]byte,
			env.getOutputSize(kernelMul2)*numSlots)
		results := stream.getCpuOutputs(env, kernelMul2, numSlots)

		// Wait on things to finish with Cuda
		// Blocks for a long time? Could be a problem?
		// Attempt 1: basic polling loop
		//for getErr := get2(stream); getErr != nil; getErr = get2(stream) {
		//	time.Sleep(200*time.Microsecond)
		//}
		err = get(stream)
		//println("Call", callId, "post get", time.Since(start))
		//start = time.Now()
		if err != nil {
			resultChan <- Mul2Result{Err: err}
			return
		}

		// Everything is OK, so let's go ahead and import the results
		result := Mul2Result{
			Slots: make([]Mul2ResultSlot, numSlots),
			Err:   nil,
		}

		offset = 0
		for i := 0; i < numSlots; i++ {
			// Output the computed result into each slot
			end := offset + bnLengthBytes
			result.Slots[i].Result = resultBuf[offset:end]
			putInt(result.Slots[i].Result,
				results[offset:end], bnLengthBytes)
			offset += bnLengthBytes
		}
		//println("Call", callId, "post arrangement", time.Since(start))

		resultChan <- result
	}()
	return resultChan
}
