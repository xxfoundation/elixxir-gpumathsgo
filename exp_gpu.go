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
)

// exp_gpu.go contains the CUDA ops for the exp operation. exp(...)
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
	numSlots := uint32(z.Len())

	// Run kernel on the inputs, simply using smaller chunks if passed
	// chunk size exceeds buffer space in stream
	stream := p.TakeStream()
	defer p.ReturnStream(stream)
	env := chooseEnv(g)
	maxSlotsExp := uint32(env.maxSlots(len(stream.cpuData), kernelPowmOdd))
	for i := uint32(0); i < numSlots; i += maxSlotsExp {
		sliceEnd := i
		// Don't slice beyond the end of the input slice
		if i+maxSlotsExp <= numSlots {
			sliceEnd += maxSlotsExp
		} else {
			sliceEnd = numSlots
		}
		err := <-exp(g, x.GetSubBuffer(i, sliceEnd), y.GetSubBuffer(i, sliceEnd), z.GetSubBuffer(i, sliceEnd), env, stream)
		if err != nil {
			return nil, err
		}
	}

	// If there were no errors, we return z
	return z, nil
}

func exp(g *cyclic.Group, x, y, result *cyclic.IntBuffer, env gpumathsEnv, stream Stream) chan error {
	// Return the result later, when the GPU job finishes
	resultChan := make(chan error, 1)
	go func() {
		// Arrange memory into stream buffers
		numSlots := uint32(x.Len())

		// TODO clean this up by implementing the
		// arrangement/dearrangement with reader/writer interfaces
		// or smth
		constants := stream.getCpuConstantsWords(env, kernelPowmOdd)
		offset := 0
		bnLengthWords := env.getWordLen()
		putBits(constants, g.GetP().Bits(), bnLengthWords)

		inputs := stream.getCpuInputsWords(env, kernelPowmOdd, int(numSlots))
		offset = 0
		for i := uint32(0); i < numSlots; i++ {
			putBits(inputs[offset:offset+bnLengthWords], x.Get(i).Bits(), bnLengthWords)
			offset += bnLengthWords
			putBits(inputs[offset:offset+bnLengthWords], y.Get(i).Bits(), bnLengthWords)
			offset += bnLengthWords
		}

		// Upload, run, wait for download
		err := env.enqueue(stream, kernelPowmOdd, int(numSlots))
		if err != nil {
			resultChan <- err
			return
		}

		// Results will be stored in this buffer
		// This intermediary copy is necessary because the byte order needs to be reversed
		results := stream.getCpuOutputsWords(env, kernelPowmOdd, int(numSlots))

		// Wait on things to finish with Cuda
		_, err = get(stream)
		if err != nil {
			resultChan <- err
			return
		}

		// Everything is OK, so let's go ahead and import the results
		offset = 0
		for i := uint32(0); i < numSlots; i++ {
			end := offset + bnLengthWords
			g.OverwriteBits(result.Get(i), results[offset:end])
			offset += bnLengthWords
		}

		resultChan <- nil
	}()
	return resultChan
}
