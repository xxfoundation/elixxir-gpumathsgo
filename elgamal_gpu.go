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
	numSlots := uint32(ecrKey.Len())

	env := chooseEnv(g)

	// Run kernel on the inputs
	stream := p.TakeStream()
	defer p.ReturnStream(stream)
	maxSlotsElGamal := uint32(env.maxSlots(len(stream.cpuData), kernelElgamal))
	for i := uint32(0); i < numSlots; i += maxSlotsElGamal {
		sliceEnd := i
		// Don't slice beyond the end of the input slice
		if i+maxSlotsElGamal <= numSlots {
			sliceEnd += maxSlotsElGamal
		} else {
			sliceEnd = numSlots
		}
		err := <-elGamal(g, key.GetSubBuffer(i, sliceEnd), privateKey.GetSubBuffer(i, sliceEnd),
			publicCypherKey, ecrKey.GetSubBuffer(i, sliceEnd), cypher.GetSubBuffer(i, sliceEnd), env, stream)
		if err != nil {
			return err
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
func elGamal(g *cyclic.Group, key, privateKey *cyclic.IntBuffer, publicCypherKey *cyclic.Int,
	ecrKey, cypher *cyclic.IntBuffer, env gpumathsEnv, stream Stream) chan error {
	// Return the result later, when the GPU job finishes
	resultChan := make(chan error, 1)
	go func() {
		// Arrange memory into stream buffers
		numSlots := uint32(key.Len())

		// TODO clean this up by implementing the
		// arrangement/dearrangement with reader/writer interfaces
		//  or smth
		constants := stream.getCpuConstantsWords(env, kernelElgamal)
		offset := 0
		bnLengthWords := env.getWordLen()
		putBits(constants[offset:offset+bnLengthWords], g.GetG().Bits(),
			bnLengthWords)
		offset += bnLengthWords
		putBits(constants[offset:offset+bnLengthWords],
			g.GetP().Bits(), bnLengthWords)
		offset += bnLengthWords
		putBits(constants[offset:offset+bnLengthWords],
			publicCypherKey.Bits(), bnLengthWords)

		inputs := stream.getCpuInputsWords(env, kernelElgamal, int(numSlots))
		offset = 0
		for i := uint32(0); i < numSlots; i++ {
			putBits(inputs[offset:offset+bnLengthWords],
				privateKey.Get(i).Bits(), bnLengthWords)
			offset += bnLengthWords
			putBits(inputs[offset:offset+bnLengthWords],
				key.Get(i).Bits(), bnLengthWords)
			offset += bnLengthWords
			putBits(inputs[offset:offset+bnLengthWords],
				ecrKey.Get(i).Bits(), bnLengthWords)
			offset += bnLengthWords
			putBits(inputs[offset:offset+bnLengthWords],
				cypher.Get(i).Bits(), bnLengthWords)
			offset += bnLengthWords
		}

		// Upload, run, wait for download
		err := env.enqueue(stream, kernelElgamal, int(numSlots))
		if err != nil {
			resultChan <- err
			return
		}
		// Results will be stored in this buffer
		results := stream.getCpuOutputsWords(env, kernelElgamal, int(numSlots))

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
			thisEcrKey := results[offset:end]
			ecrKeyCopy := make(large.Bits, len(thisEcrKey))
			putBits(ecrKeyCopy, results[offset:end],
				bnLengthWords)
			g.SetBits(ecrKey.Get(i), ecrKeyCopy)
			offset += bnLengthWords
			end = offset + bnLengthWords
			thisCypher := results[offset:end]
			cypherCopy := make(large.Bits, len(thisCypher))
			putBits(cypherCopy, thisCypher, bnLengthWords)
			g.SetBits(cypher.Get(i), cypherCopy)
			offset += bnLengthWords
		}

		resultChan <- nil
	}()
	return resultChan
}
