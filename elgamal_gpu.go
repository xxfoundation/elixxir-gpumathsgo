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

// elgamal_gpu.go contains the CUDA ops for the ElGamal operation. ElGamal(...)
// performs the actual call into the library and ElGamalChunk implements
// the streaming interface function called by the server implementation.

const (
	kernelElgamal = C.KERNEL_ELGAMAL
)

// Precondition: All int buffers must have the same length
// Perform the ElGamal operation on two int buffers
var ElGamalChunk ElGamalChunkPrototype = func(p *StreamPool, g *cyclic.Group,
	key, privateKey *cyclic.IntBuffer, publicCypherKey *cyclic.Int,
	ecrKey, cypher *cyclic.IntBuffer) error {
	// Populate ElGamal inputs
	numSlots := ecrKey.Len()
	input := ElGamalInput{
		Slots:           make([]ElGamalInputSlot, numSlots),
		PublicCypherKey: publicCypherKey.Bytes(),
		Prime:           g.GetPBytes(),
		G:               g.GetG().Bytes(),
	}
	for i := uint32(0); i < uint32(numSlots); i++ {
		input.Slots[i] = ElGamalInputSlot{
			PrivateKey: privateKey.Get(i).Bytes(),
			Key:        key.Get(i).Bytes(),
			EcrKey:     ecrKey.Get(i).Bytes(),
			Cypher:     cypher.Get(i).Bytes(),
		}
	}

	// Run kernel on the inputs
	stream := p.TakeStream()
	defer p.ReturnStream(stream)
	for i := 0; i < numSlots; i += stream.maxSlotsElGamal {
		sliceEnd := i
		// Don't slice beyond the end of the input slice
		if i+stream.maxSlotsElGamal <= numSlots {
			sliceEnd += stream.maxSlotsElGamal
		} else {
			sliceEnd = numSlots
		}
		thisInput := ElGamalInput{
			Slots:           input.Slots[i:sliceEnd],
			Prime:           input.Prime,
			G:               input.G,
			PublicCypherKey: input.PublicCypherKey,
		}
		result := <-ElGamal(thisInput, stream)
		if result.Err != nil {
			return result.Err
		}
		// Populate with results
		for j := range result.Slots {
			g.SetBytes(ecrKey.Get(uint32(i+j)),
				result.Slots[j].EcrKey)
			g.SetBytes(cypher.Get(uint32(i+j)),
				result.Slots[j].Cypher)
		}
	}

	return nil
}

// ElGamal runs the op on the GPU
// publicCypherKey and prime should be byte slices obtained by running
// .Bytes() on the large int
// The resulting byte slice should be trimmed and should be less than or
// equal to the length of the template-instantiated BN on the GPU.
// bnLength is a length in bits
// TODO validate BN length in code (i.e. pick kernel variants based on bn length)
func ElGamal(input ElGamalInput, stream Stream) chan ElGamalResult {
	// Return the result later, when the GPU job finishes
	resultChan := make(chan ElGamalResult, 1)
	go func() {
		validateElgamalInput(input, stream)

		// Arrange memory into stream buffers
		numSlots := len(input.Slots)

		// TODO clean this up by implementing the
		// arrangement/dearrangement with reader/writer interfaces
		//  or smth
		constants := toSlice(C.getCpuConstants(stream.s),
			int(C.getConstantsSize(kernelElgamal)))
		offset := 0
		putInt(constants[offset:offset+bnLengthBytes], input.G,
			bnLengthBytes)
		offset += bnLengthBytes
		putInt(constants[offset:offset+bnLengthBytes],
			input.Prime, bnLengthBytes)
		offset += bnLengthBytes
		putInt(constants[offset:offset+bnLengthBytes],
			input.PublicCypherKey, bnLengthBytes)

		inputs := toSlice(C.getCpuInputs(stream.s, kernelElgamal),
			int(C.getInputSize(kernelElgamal))*numSlots)
		offset = 0
		for i := 0; i < numSlots; i++ {
			putInt(inputs[offset:offset+bnLengthBytes],
				input.Slots[i].PrivateKey, bnLengthBytes)
			offset += bnLengthBytes
			putInt(inputs[offset:offset+bnLengthBytes],
				input.Slots[i].Key, bnLengthBytes)
			offset += bnLengthBytes
			putInt(inputs[offset:offset+bnLengthBytes],
				input.Slots[i].EcrKey, bnLengthBytes)
			offset += bnLengthBytes
			putInt(inputs[offset:offset+bnLengthBytes],
				input.Slots[i].Cypher, bnLengthBytes)
			offset += bnLengthBytes
		}

		// Upload, run, wait for download
		err := put(stream, kernelElgamal, numSlots)
		if err != nil {
			resultChan <- ElGamalResult{Err: err}
			return
		}
		err = run(stream)
		if err != nil {
			resultChan <- ElGamalResult{Err: err}
			return
		}
		err = download(stream)
		if err != nil {
			resultChan <- ElGamalResult{Err: err}
			return
		}

		// Results will be stored in this buffer
		resultBuf := make([]byte,
			C.getOutputSize(kernelElgamal)*(C.size_t)(numSlots))
		results := toSlice(C.getCpuOutputs(stream.s), len(resultBuf))

		// Wait on things to finish with Cuda
		err = get(stream)
		if err != nil {
			resultChan <- ElGamalResult{Err: err}
			return
		}

		// Everything is OK, so let's go ahead and import the results
		result := ElGamalResult{
			Slots: make([]ElGamalResultSlot, numSlots),
			Err:   nil,
		}

		offset = 0
		for i := 0; i < numSlots; i++ {
			end := offset + bnLengthBytes
			result.Slots[i].EcrKey = resultBuf[offset:end]
			putInt(result.Slots[i].EcrKey, results[offset:end],
				bnLengthBytes)
			offset += bnLengthBytes
			end = offset + bnLengthBytes
			result.Slots[i].Cypher = resultBuf[offset:end]
			putInt(result.Slots[i].Cypher, results[offset:end],
				bnLengthBytes)
			offset += bnLengthBytes
		}

		resultChan <- result
	}()
	return resultChan
}

// Bounds check to make sure that the stream can take all the inputs
func validateElgamalInput(input ElGamalInput, stream Stream) {
	if len(input.Slots) > stream.maxSlotsElGamal {
		// This can only happen because of user error (unlike Cuda
		// problems), so panic to make the error apparent
		log.Panicf(fmt.Sprintf("%v slots is more than this stream's "+
			"max of %v for ElGamal kernel",
			len(input.Slots), stream.maxSlotsElGamal))
	}
}

// Four numbers per input
// Returns size in bytes
func getInputsSizeElgamal() int {
	return int(C.getInputSize(kernelElgamal))
}

// Two numbers per output
// Returns size in bytes
func getOutputsSizeElgamal() int {
	return int(C.getOutputSize(kernelElgamal))
}

// Three numbers (g, prime, publicCypherKey)
// Returns size in bytes
func getConstantsSizeElgamal() int {
	return int(C.getConstantsSize(kernelElgamal))
}
