////////////////////////////////////////////////////////////////////////////////
// Copyright © 2019 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

//+build linux,gpu

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

// reveal_gpu.go contains the CUDA ops for the Reveal operation. Reveal(...)
// performs the actual call into the library and RevealChunk implements
// the streaming interface function called by the server implementation.

const kernelReveal = C.KERNEL_REVEAL

// RevealChunk performs the Reveal operation on the cypher payloads
// Precondition: All int buffers must have the same length
var RevealChunk RevealChunkPrototype = func(p *StreamPool, g *cyclic.Group,
	publicCypherKey *cyclic.Int, cypher *cyclic.IntBuffer) error {
	// Populate Reveal inputs
	numSlots := cypher.Len()
	input := RevealInput{
		Slots:           make([]RevealInputSlot, numSlots),
		PublicCypherKey: publicCypherKey.Bytes(),
		Prime:           g.GetPBytes(),
	}
	for i := uint32(0); i < uint32(numSlots); i++ {
		input.Slots[i] = RevealInputSlot{
			Cypher: cypher.Get(i).Bytes(),
		}
	}

	// Run kernel on the inputs
	stream := p.TakeStream()
	defer p.ReturnStream(stream)
	for i := 0; i < numSlots; i += stream.maxSlotsReveal {
		sliceEnd := i
		// Don't slice beyond the end of the input slice
		if i+stream.maxSlotsReveal <= numSlots {
			sliceEnd += stream.maxSlotsReveal
		} else {
			sliceEnd = numSlots
		}
		thisInput := RevealInput{
			Slots:           input.Slots[i:sliceEnd],
			Prime:           input.Prime,
			PublicCypherKey: input.PublicCypherKey,
		}
		result := <-Reveal(thisInput, stream)
		if result.Err != nil {
			return result.Err
		}
		// Populate with results
		for j := range result.Slots {
			g.SetBytes(cypher.Get(uint32(i+j)),
				result.Slots[j].Cypher)
		}
	}

	return nil
}

// Bounds check to make sure that the stream can take all the inputs
func validateRevealInput(input RevealInput, stream Stream) {
	if len(input.Slots) > stream.maxSlotsReveal {
		// This can only happen because of user error (unlike Cuda
		// problems), so panic to make the error apparent
		log.Panicf(fmt.Sprintf("%v slots is more than this stream's "+
			"max of %v for Reveal kernel",
			len(input.Slots), stream.maxSlotsReveal))
	}
}

// Reveal runs the reveal operation on cypher payloads inside the GPU
// NOTE: publicCypherKey and prime should be byte slices obtained by running
//       .Bytes() on the large int
// The resulting byte slice should be trimmed and should be less than or
// equal to the length of the template-instantiated BN on the GPU.
// bnLength is a length in bits
// TODO validate BN length in code (i.e. pick kernel variants based on bn length)
func Reveal(input RevealInput, stream Stream) chan RevealResult {
	// Return the result later, when the GPU job finishes
	resultChan := make(chan RevealResult, 1)
	go func() {
		validateRevealInput(input, stream)

		// Arrange memory into stream buffers
		numSlots := len(input.Slots)

		// TODO clean this up by implementing the
		// arrangement/dearrangement with reader/writer interfaces
		// or smth
		constants := toSlice(C.getCpuConstants(stream.s),
			int(C.getConstantsSize(kernelReveal)))
		offset := 0
		// Prime
		putInt(constants[offset:offset+bnLengthBytes],
			input.Prime, bnLengthBytes)
		offset += bnLengthBytes
		// The compted PublicCypherKey
		putInt(constants[offset:offset+bnLengthBytes],
			input.PublicCypherKey, bnLengthBytes)

		inputs := toSlice(C.getCpuInputs(stream.s, kernelReveal),
			int(C.getInputSize(kernelReveal))*numSlots)
		offset = 0
		for i := 0; i < numSlots; i++ {
			// Put the CypherPayload for this slot
			putInt(inputs[offset:offset+bnLengthBytes],
				input.Slots[i].Cypher, bnLengthBytes)
			offset += bnLengthBytes
		}

		// Upload, run, wait for download
		err := put(stream, kernelReveal, numSlots)
		if err != nil {
			resultChan <- RevealResult{Err: err}
			return
		}
		err = run(stream)
		if err != nil {
			resultChan <- RevealResult{Err: err}
			return
		}
		err = download(stream)
		if err != nil {
			resultChan <- RevealResult{Err: err}
			return
		}

		// Results will be stored in this buffer
		resultBuf := make([]byte,
			C.getOutputSize(kernelReveal)*(C.size_t)(numSlots))
		results := toSlice(C.getCpuOutputs(stream.s), len(resultBuf))

		// Wait on things to finish with Cuda
		err = get(stream)
		if err != nil {
			resultChan <- RevealResult{Err: err}
			return
		}

		// Everything is OK, so let's go ahead and import the results
		result := RevealResult{
			Slots: make([]RevealResultSlot, numSlots),
			Err:   nil,
		}

		offset = 0
		for i := 0; i < numSlots; i++ {
			offsetend := offset + bnLengthBytes
			result.Slots[i].Cypher = resultBuf[offset:offsetend]
			putInt(result.Slots[i].Cypher,
				results[offset:offsetend], bnLengthBytes)
			offset += bnLengthBytes
		}

		resultChan <- result
	}()
	return resultChan
}

// Helper functions
// 1 number per input
// Returns size in bytes
func getInputsSizeReveal() int {
	return int(C.getInputSize(kernelReveal))
}

// Returns size in bytes
// 1 number per output
func getOutputsSizeReveal() int {
	return int(C.getOutputSize(kernelReveal))
}

// 2 numbers (prime, publicCypherKey)
// Returns size in bytes
func getConstantsSizeReveal() int {
	return int(C.getConstantsSize(kernelReveal))
}
