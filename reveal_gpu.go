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
	jww "github.com/spf13/jwalterweatherman"
	"git.xx.network/elixxir/crypto/cyclic"
)

// reveal_gpu.go contains the CUDA ops for the reveal operation. reveal(...)
// performs the actual call into the library and RevealChunk implements
// the streaming interface function called by the server implementation.

const kernelReveal = C.KERNEL_REVEAL

// RevealChunk performs the reveal operation on the cypher payloads
// Precondition: All int buffers must have the same length
var RevealChunk RevealChunkPrototype = func(p *StreamPool, g *cyclic.Group,
	publicCypherKey *cyclic.Int, cypher *cyclic.IntBuffer, result *cyclic.IntBuffer) error {
	// Populate reveal inputs
	numSlots := uint32(cypher.Len())

	env := chooseEnv(g)

	// Run kernel on the inputs
	stream := p.TakeStream()
	defer p.ReturnStream(stream)
	maxSlotsReveal := uint32(env.maxSlots(len(stream.cpuData), kernelReveal))
	if numSlots > maxSlotsReveal {
		jww.WARN.Printf("Running multiple kernels for RevealChunk. Performance may be degraded")
	}
	for i := uint32(0); i < numSlots; i += maxSlotsReveal {
		sliceEnd := i
		// Don't slice beyond the end of the input slice
		if i+maxSlotsReveal <= numSlots {
			sliceEnd += maxSlotsReveal
		} else {
			sliceEnd = numSlots
		}
		err := <-reveal(g, publicCypherKey, cypher.GetSubBuffer(i, sliceEnd), result.GetSubBuffer(i, sliceEnd), env, stream)
		if err != nil {
			return err
		}
	}

	return nil
}

// reveal runs the reveal operation on cypher payloads inside the GPU
// NOTE: publicCypherKey and prime should be byte slices obtained by running
//       .Bytes() on the large int
// The resulting byte slice should be trimmed and should be less than or
// equal to the length of the template-instantiated BN on the GPU.
// bnLength is a length in bits
// TODO validate BN length in code (i.e. pick kernel variants based on bn length)
func reveal(g *cyclic.Group, publicCypherKey *cyclic.Int, cypher *cyclic.IntBuffer, result *cyclic.IntBuffer, env gpumathsEnv, stream Stream) chan error {
	// Return the result later, when the GPU job finishes
	errors := make(chan error, 1)
	go func() {
		// Arrange memory into stream buffers
		numSlots := uint32(cypher.Len())

		constants := stream.getCpuConstantsWords(env, kernelReveal)
		offset := 0
		// Prime
		bnLengthWords := env.getWordLen()
		putBits(constants[offset:offset+bnLengthWords], g.GetP().Bits(), bnLengthWords)
		offset += bnLengthWords
		// The compted PublicCypherKey
		putBits(constants[offset:offset+bnLengthWords], publicCypherKey.Bits(), bnLengthWords)

		inputs := stream.getCpuInputsWords(env, kernelReveal, int(numSlots))
		offset = 0
		for i := uint32(0); i < numSlots; i++ {
			// Put the CypherPayload for this slot
			putBits(inputs[offset:offset+bnLengthWords], cypher.Get(i).Bits(), bnLengthWords)
			offset += bnLengthWords
		}

		// Upload, run, wait for download
		err := env.enqueue(stream, kernelReveal, int(numSlots))
		if err != nil {
			errors <- err
			return
		}

		// Results will be stored in this buffer
		results := stream.getCpuOutputsWords(env, kernelReveal, int(numSlots))

		// Wait on things to finish with Cuda
		err = get(stream)
		if err != nil {
			errors <- err
			return
		}

		offset = 0
		for i := uint32(0); i < numSlots; i++ {
			offsetend := offset + bnLengthWords
			g.OverwriteBits(result.Get(i), results[offset:offsetend])
			offset += bnLengthWords
		}

		errors <- nil
	}()
	return errors
}
