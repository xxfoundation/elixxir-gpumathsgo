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

	env := chooseEnv(g)

	// Run kernel on the inputs
	stream := p.TakeStream()
	defer p.ReturnStream(stream)
	maxSlotsReveal := env.maxSlots(len(stream.cpuData), kernelReveal)
	for i := 0; i < numSlots; i += maxSlotsReveal {
		sliceEnd := i
		// Don't slice beyond the end of the input slice
		if i+maxSlotsReveal <= numSlots {
			sliceEnd += maxSlotsReveal
		} else {
			sliceEnd = numSlots
		}
		thisInput := RevealInput{
			Slots:           input.Slots[i:sliceEnd],
			Prime:           input.Prime,
			PublicCypherKey: input.PublicCypherKey,
		}
		result := <-Reveal(thisInput, env, stream)
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
func validateRevealInput(input RevealInput, env gpumathsEnv, stream Stream) {
	maxSlotsReveal := env.maxSlots(len(stream.cpuData), kernelReveal)
	if len(input.Slots) > maxSlotsReveal {
		// This can only happen because of user error (unlike Cuda
		// problems), so panic to make the error apparent
		log.Panicf(fmt.Sprintf("%v slots is more than this stream's "+
			"max of %v for Reveal kernel",
			len(input.Slots), maxSlotsReveal))
	}
}

// Reveal runs the reveal operation on cypher payloads inside the GPU
// NOTE: publicCypherKey and prime should be byte slices obtained by running
//       .Bytes() on the large int
// The resulting byte slice should be trimmed and should be less than or
// equal to the length of the template-instantiated BN on the GPU.
// bnLength is a length in bits
// TODO validate BN length in code (i.e. pick kernel variants based on bn length)
func Reveal(input RevealInput, env gpumathsEnv, stream Stream) chan RevealResult {
	// Return the result later, when the GPU job finishes
	resultChan := make(chan RevealResult, 1)
	go func() {
		validateRevealInput(input, env, stream)

		// Arrange memory into stream buffers
		numSlots := len(input.Slots)

		// TODO clean this up by implementing the
		// arrangement/dearrangement with reader/writer interfaces
		// or smth
		constants := stream.getCpuConstants(env, kernelReveal)
		offset := 0
		// Prime
		bnLengthBytes := env.getByteLen()
		putInt(constants[offset:offset+bnLengthBytes],
			input.Prime, bnLengthBytes)
		offset += bnLengthBytes
		// The compted PublicCypherKey
		putInt(constants[offset:offset+bnLengthBytes],
			input.PublicCypherKey, bnLengthBytes)

		inputs := stream.getCpuInputs(env, kernelReveal, numSlots)
		offset = 0
		for i := 0; i < numSlots; i++ {
			// Put the CypherPayload for this slot
			putInt(inputs[offset:offset+bnLengthBytes],
				input.Slots[i].Cypher, bnLengthBytes)
			offset += bnLengthBytes
		}

		// Upload, run, wait for download
		err := env.put(stream, kernelReveal, numSlots)
		if err != nil {
			resultChan <- RevealResult{Err: err}
			return
		}
		err = env.run(stream)
		if err != nil {
			resultChan <- RevealResult{Err: err}
			return
		}
		err = env.download(stream)
		if err != nil {
			resultChan <- RevealResult{Err: err}
			return
		}

		// Results will be stored in this buffer
		resultBuf := make([]byte,
			env.getOutputSize(kernelReveal)*numSlots)
		results := stream.getCpuOutputs(env, kernelReveal, numSlots)

		// Wait on things to finish with Cuda
		_, err = get(stream)
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
