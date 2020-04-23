////////////////////////////////////////////////////////////////////////////////
// Copyright © 2019 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

//+build linux,cuda

package gpumaths

/*#cgo LDFLAGS: -Llib -lpowmosm75 -Wl,-rpath -Wl,./lib:/opt/elixxir/lib
#cgo CFLAGS: -I./cgbnBindings/powm -I/opt/elixxir/include
#include <powm_odd_export.h>
*/
import "C"

import (
	"fmt"
	"gitlab.com/elixxir/crypto/cyclic"
	"log"
)

// strip_gpu.go contains the CUDA ops for the Strip operation. Strip(...)
// performs the actual call into the library and StripChunk implements
// the streaming interface function called by the server implementation.

const kernelStrip = C.KERNEL_STRIP

// StripChunk performs the Strip operation on the cypher and precomputation
// payloads
// Precondition: All int buffers must have the same length
var StripChunk StripChunkPrototype = func(p *StreamPool, g *cyclic.Group,
	publicCypherKey *cyclic.Int,
	precomputation []*cyclic.Int, cypher *cyclic.IntBuffer) error {
	// Populate Strip inputs
	numSlots := cypher.Len()
	input := StripInput{
		Slots:           make([]StripInputSlot, numSlots),
		PublicCypherKey: publicCypherKey.Bytes(),
		Prime:           g.GetPBytes(),
	}
	for i := uint32(0); i < uint32(numSlots); i++ {
		input.Slots[i] = StripInputSlot{
			Precomputation: precomputation[i].Bytes(),
			Cypher:         cypher.Get(i).Bytes(),
		}
	}

	// Run kernel on the inputs
	stream := p.TakeStream()
	defer p.ReturnStream(stream)
	for i := 0; i < numSlots; i += stream.maxSlotsStrip {
		sliceEnd := i
		// Don't slice beyond the end of the input slice
		if i+stream.maxSlotsStrip <= numSlots {
			sliceEnd += stream.maxSlotsStrip
		} else {
			sliceEnd = numSlots
		}
		thisInput := StripInput{
			Slots:           input.Slots[i:sliceEnd],
			Prime:           input.Prime,
			PublicCypherKey: input.PublicCypherKey,
		}
		result := <-Strip(thisInput, stream)
		if result.Err != nil {
			return result.Err
		}
		// Populate with results
		for j := range result.Slots {
			g.SetBytes(cypher.Get(uint32(i+j)),
				result.Slots[j].Precomputation)
		}
	}

	return nil
}

// Bounds check to make sure that the stream can take all the inputs
func validateStripInput(input StripInput, stream Stream) {
	if len(input.Slots) > stream.maxSlotsStrip {
		// This can only happen because of user error (unlike Cuda
		// problems), so panic to make the error apparent
		log.Panicf(fmt.Sprintf("%v slots is more than this stream's "+
			"max of %v for Strip kernel",
			len(input.Slots), stream.maxSlotsStrip))
	}
}

// Strip runs the strip operation on precomputation and cypher payloads inside
// the GPU
// NOTE: publicCypherKey and prime should be byte slices obtained by running
//       .Bytes() on the large int
// The resulting byte slice should be trimmed and should be less than or
// equal to the length of the template-instantiated BN on the GPU.
// bnLength is a length in bits
// TODO validate BN length in code (i.e. pick kernel variants based on bn
// length)
func Strip(input StripInput, stream Stream) chan StripResult {
	// Return the result later, when the GPU job finishes
	resultChan := make(chan StripResult, 1)
	go func() {
		validateStripInput(input, stream)

		// Arrange memory into stream buffers
		numSlots := len(input.Slots)

		// TODO clean this up by implementing the
		// arrangement/dearrangement with reader/writer interfaces
		// or smth
		constants := toSlice(C.getCpuConstants(stream.s),
			int(C.getConstantsSize(kernelStrip)))
		offset := 0
		// Prime
		putInt(constants[offset:offset+bnLengthBytes],
			input.Prime, bnLengthBytes)
		offset += bnLengthBytes
		// The compted PublicCypherKey
		putInt(constants[offset:offset+bnLengthBytes],
			input.PublicCypherKey, bnLengthBytes)

		inputs := toSlice(C.getCpuInputs(stream.s, kernelStrip),
			int(C.getInputSize(kernelStrip))*numSlots)
		offset = 0
		for i := 0; i < numSlots; i++ {
			// Put the PrecomputationPayload for this slot
			putInt(inputs[offset:offset+bnLengthBytes],
				input.Slots[i].Precomputation, bnLengthBytes)
			offset += bnLengthBytes
			// Put the CypherPayload for this slot
			putInt(inputs[offset:offset+bnLengthBytes],
				input.Slots[i].Cypher, bnLengthBytes)
			offset += bnLengthBytes
		}

		// Upload, run, wait for download
		err := put(stream, kernelStrip, numSlots)
		if err != nil {
			resultChan <- StripResult{Err: err}
			return
		}
		err = run(stream)
		if err != nil {
			resultChan <- StripResult{Err: err}
			return
		}
		err = download(stream)
		if err != nil {
			resultChan <- StripResult{Err: err}
			return
		}

		// Results will be stored in this buffer
		resultBuf := make([]byte,
			C.getOutputSize(kernelStrip)*(C.size_t)(numSlots))
		results := toSlice(C.getCpuOutputs(stream.s), len(resultBuf))

		// Wait on things to finish with Cuda
		err = get(stream)
		if err != nil {
			resultChan <- StripResult{Err: err}
			return
		}

		// Everything is OK, so let's go ahead and import the results
		result := StripResult{
			Slots: make([]StripResultSlot, numSlots),
			Err:   nil,
		}

		offset = 0
		for i := 0; i < numSlots; i++ {
			// Output the computed Precomputation (EncryptedKeys)
			// for the payload into each slot
			end := offset + bnLengthBytes
			result.Slots[i].Precomputation = resultBuf[offset:end]
			putInt(result.Slots[i].Precomputation,
				results[offset:end], bnLengthBytes)
			offset += bnLengthBytes
		}

		resultChan <- result
	}()
	return resultChan
}

// Helper functions
// 2 numbers per input
// Returns size in bytes
func getInputsSizeStrip() int {
	return int(C.getInputSize(kernelStrip))
}

// 1 number per output
// Returns size in bytes
func getOutputsSizeStrip() int {
	return int(C.getOutputSize(kernelStrip))
}

// Two numbers (prime, publicCypherKey)
// Returns size in bytes
func getConstantsSizeStrip() int {
	return int(C.getConstantsSize(kernelStrip))
}
