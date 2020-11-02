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
	env := chooseEnv(g)
	maxSlotsExp := env.maxSlots(len(stream.cpuData), kernelPowmOdd)
	for i := 0; i < numSlots; i += maxSlotsExp {
		sliceEnd := i
		// Don't slice beyond the end of the input slice
		if i+maxSlotsExp <= numSlots {
			sliceEnd += maxSlotsExp
		} else {
			sliceEnd = numSlots
		}
		thisInput := ExpInput{
			Slots:   input.Slots[i:sliceEnd],
			Modulus: input.Modulus,
		}
		result := <-Exp(thisInput, env, stream)
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

func Exp(input ExpInput, env gpumathsEnv, stream Stream) chan ExpResult {
	// Return the result later, when the GPU job finishes
	resultChan := make(chan ExpResult, 1)
	go func() {
		validateExpInput(input, env, stream)

		// Arrange memory into stream buffers
		numSlots := len(input.Slots)

		// TODO clean this up by implementing the
		// arrangement/dearrangement with reader/writer interfaces
		// or smth
		constants := stream.getCpuConstants(env, kernelPowmOdd)
		offset := 0
		bnLengthBytes := env.getByteLen()
		putInt(constants[offset:offset+bnLengthBytes], input.Modulus,
			bnLengthBytes)

		inputs := stream.getCpuInputs(env, kernelPowmOdd, numSlots)
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
		err := env.put(stream, kernelPowmOdd, numSlots)
		if err != nil {
			resultChan <- ExpResult{Err: err}
			return
		}
		err = env.run(stream)
		if err != nil {
			resultChan <- ExpResult{Err: err}
			return
		}
		err = env.download(stream)
		if err != nil {
			resultChan <- ExpResult{Err: err}
			return
		}

		// Results will be stored in this buffer
		// This intermediary copy is necessary because the byte order needs to be reversed
		resultBuf := make([]byte,
			env.getOutputSize(kernelPowmOdd)*numSlots)
		results := stream.getCpuOutputs(env, kernelPowmOdd, numSlots)

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

// Bounds check to make sure that the stream can take all the inputs
func validateExpInput(input ExpInput, env gpumathsEnv, stream Stream) {
	maxSlotsExp := env.maxSlots(len(stream.cpuData), kernelPowmOdd)
	if len(input.Slots) > maxSlotsExp {
		// This can only happen because of user error (unlike Cuda
		// problems), so panic to make the error apparent
		log.Panicf(fmt.Sprintf("%v slots is more than this stream's "+
			"max of %v for Exp kernel",
			len(input.Slots), maxSlotsExp))
	}
}
