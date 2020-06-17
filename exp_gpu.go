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

// exp_gpu.go contains the CUDA ops for the Exp operation. Exp(...)
// performs the actual call into the library and ExpChunk implements
// the streaming interface function called by the server implementation.

const (
	kernelPowmOdd = C.KERNEL_POWM_ODD
)

// ExpChunk Performs exponentiation for two operands and place the result in z
// (which is also returned)
// Using this function doesn't allow you to do other things while waiting
// on the kernel to finish
var ExpChunk ExpChunkPrototype = func(p *StreamPool, g *cyclic.Group,
	x, y, z *cyclic.IntBuffer) (*cyclic.IntBuffer, error) {
	// Populate exp inputs
	numSlots := z.Len()
	input := ExpInput{
		Slots:   make([]ExpInputSlot, numSlots),
		Modulus: g.GetPBytes(),
	}
	for i := uint32(0); i < uint32(numSlots); i++ {
		input.Slots[i] = ExpInputSlot{
			Base:     x.Get(i).Bytes(),
			Exponent: y.Get(i).Bytes(),
		}
	}

	// Run kernel on the inputs, simply using smaller chunks if passed
	// chunk size exceeds buffer space in stream
	stream := p.TakeStream()
	defer p.ReturnStream(stream)
	for i := 0; i < numSlots; i += stream.maxSlotsExp {
		sliceEnd := i
		// Don't slice beyond the end of the input slice
		if i+stream.maxSlotsExp <= numSlots {
			sliceEnd += stream.maxSlotsExp
		} else {
			sliceEnd = numSlots
		}
		thisInput := ExpInput{
			Slots:   input.Slots[i:sliceEnd],
			Modulus: input.Modulus,
		}
		result := <-Exp(thisInput, stream)
		if result.Err != nil {
			return z, result.Err
		}
		// Populate with results
		for j := range result.Results {
			g.SetBytes(z.Get(uint32(i+j)), result.Results[j])
		}
	}

	// If there were no errors, we return z
	return z, nil
}

// Exp runs exponentiation on the GPU
// Precondition: Modulus must be odd
func Exp(input ExpInput, stream Stream) chan ExpResult {
	// Return the result later, when the GPU job finishes
	resultChan := make(chan ExpResult, 1)
	go func() {
		validateExpInput(input, stream)

		// Arrange memory into stream buffers
		numSlots := len(input.Slots)

		// TODO clean this up by implementing the
		// arrangement/dearrangement with reader/writer interfaces
		// or smth
		constants := toSlice(C.getCpuConstants(stream.s),
			int(C.getConstantsSize(kernelPowmOdd)))
		offset := 0
		putInt(constants[offset:offset+bnLengthBytes], input.Modulus,
			bnLengthBytes)

		inputs := toSlice(C.getCpuInputs(stream.s, kernelPowmOdd),
			int(C.getInputSize(kernelPowmOdd))*numSlots)
		offset = 0
		for i := 0; i < numSlots; i++ {
			putInt(inputs[offset:offset+bnLengthBytes],
				input.Slots[i].Base, bnLengthBytes)
			offset += bnLengthBytes
			putInt(inputs[offset:offset+bnLengthBytes],
				input.Slots[i].Exponent, bnLengthBytes)
			offset += bnLengthBytes
		}

		// Upload, run, wait for download
		err := put(stream, kernelPowmOdd, numSlots)
		if err != nil {
			resultChan <- ExpResult{Err: err}
			return
		}
		err = run(stream)
		if err != nil {
			resultChan <- ExpResult{Err: err}
			return
		}
		err = download(stream)
		if err != nil {
			resultChan <- ExpResult{Err: err}
			return
		}

		// Results will be stored in this buffer
		resultBuf := make([]byte,
			C.getOutputSize(kernelPowmOdd)*(C.size_t)(numSlots))
		results := toSlice(C.getCpuOutputs(stream.s), len(resultBuf))

		// Wait on things to finish with Cuda
		err = get(stream)
		if err != nil {
			resultChan <- ExpResult{Err: err}
			return
		}

		// Everything is OK, so let's go ahead and import the results
		result := ExpResult{
			Results: make([][]byte, numSlots),
			Err:     nil,
		}

		offset = 0
		for i := 0; i < numSlots; i++ {
			end := offset + bnLengthBytes
			result.Results[i] = resultBuf[offset:end]
			putInt(result.Results[i], results[offset:end],
				bnLengthBytes)
			offset += bnLengthBytes
		}

		resultChan <- result
	}()
	return resultChan
}

// Checks if there's a number of exp slots that the stream can handle
func validateExpInput(input ExpInput, stream Stream) {
	if len(input.Slots) > stream.maxSlotsExp {
		log.Panicf(fmt.Sprintf("%v slots is more than this stream's "+
			"max of %v for Exp kernel",
			len(input.Slots), stream.maxSlotsExp))
	}
}

// Two numbers per input
// Returns size in bytes
func getInputsSizePowm4096() int {
	return int(C.getInputSize(kernelPowmOdd))
}

// One number per output
// Returns size in bytes
func getOutputsSizePowm4096() int {
	return int(C.getOutputSize(kernelPowmOdd))
}

// One number (prime)
// Returns size in bytes
func getConstantsSizePowm4096() int {
	return int(C.getConstantsSize(kernelPowmOdd))
}
