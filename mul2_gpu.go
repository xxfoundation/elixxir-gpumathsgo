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
	"math/rand"
	"time"
)

// mul2_gpu.go contains the CUDA ops for the mul2 operation. mul2(...)
// performs the actual call into the library and Mul2Chunk implements
// the streaming interface function called by the server implementation.

const kernelMul2 = C.KERNEL_MUL2

// This interface provides compatibility with the underlying mul2 method
// Int buffers and slices can both be used to implement this interface
type intGetter interface {
	Get(index uint32) *cyclic.Int
	Len() int
}

type intSlice []*cyclic.Int

// Implement intGetter with cyclic int slice
func (s intSlice) Get(index uint32) *cyclic.Int {
	return s[index]
}

func (s intSlice) Len() int {
	return len(s)
}

// Mul2Chunk performs the mul2 operation on the cypher and precomputation
// payloads
// Precondition: All int buffers must have the same length
var Mul2Chunk Mul2ChunkPrototype = func(p *StreamPool, g *cyclic.Group,
	x *cyclic.IntBuffer, y *cyclic.IntBuffer, results *cyclic.IntBuffer) error {
	// Populate mul2 inputs
	numSlots := uint32(x.Len())

	// Run kernel on the inputs
	stream := p.TakeStream()
	defer p.ReturnStream(stream)
	env := chooseEnv(g)
	maxSlotsMul2 := uint32(env.maxSlots(len(stream.cpuData), kernelMul2))
	for i := uint32(0); i < numSlots; i += maxSlotsMul2 {
		sliceEnd := i
		// Don't slice beyond the end of the input slice
		if i+maxSlotsMul2 <= numSlots {
			sliceEnd += maxSlotsMul2
		} else {
			sliceEnd = numSlots
		}
		err := <-mul2(g, x.GetSubBuffer(i, sliceEnd), y.GetSubBuffer(i, sliceEnd), results.GetSubBuffer(i, sliceEnd), env, stream)
		if err != nil {
			return err
		}
	}

	return nil
}

var Mul2Slice Mul2SlicePrototype = func(p *StreamPool, g *cyclic.Group, x *cyclic.IntBuffer, y, result []*cyclic.Int) error {
	// Populate mul2 inputs
	numSlots := uint32(x.Len())

	// Run kernel on the inputs
	stream := p.TakeStream()
	defer p.ReturnStream(stream)
	env := chooseEnv(g)
	maxSlotsMul2 := uint32(env.maxSlots(len(stream.cpuData), kernelMul2))
	for i := uint32(0); i < numSlots; i += maxSlotsMul2 {
		sliceEnd := i
		// Don't slice beyond the end of the input slice
		if i+maxSlotsMul2 <= numSlots {
			sliceEnd += maxSlotsMul2
		} else {
			sliceEnd = numSlots
		}
		err := <-mul2(g, x.GetSubBuffer(i, sliceEnd), intSlice(y[i:sliceEnd]), intSlice(result[i:sliceEnd]), env, stream)
		if err != nil {
			return err
		}
	}

	return nil
}

// mul2 runs the mul2 operation on precomputation and cypher payloads inside
// the GPU
// NOTE: publicCypherKey and prime should be byte slices obtained by running
//       .Bytes() on the large int
// The resulting byte slice should be trimmed and should be less than or
// equal to the length of the template-instantiated BN on the GPU.
// bnLength is a length in bits
// puts output in results int buffer
func mul2(g *cyclic.Group, x intGetter, y intGetter, results intGetter, env gpumathsEnv, stream Stream) chan error {
	debugPrint := false
	callId := rand.Intn(9999)
	start := time.Now()
	//if debugPrint {
	//	println("starting mul2 call", callId, "with", len(input.Slots), "slots at", start.String())
	//}
	start = time.Now()
	//Return the result later, when the GPU job finishes
	resultChan := make(chan error, 1)
	go func() {
		// Arrange memory into stream buffers
		numSlots := uint32(x.Len())

		// TODO clean this up by implementing the
		// arrangement/dearrangement with reader/writer interfaces
		// or smth
		constants := stream.getCpuConstantsWords(env, kernelMul2)
		bnLengthWords := env.getWordLen()
		putBits(constants, g.GetP().Bits(), bnLengthWords)
		offset := 0
		offset += bnLengthWords

		inputs := stream.getCpuInputsWords(env, kernelMul2, int(numSlots))
		offset = 0
		for i := uint32(0); i < numSlots; i++ {
			// Put the first operand for this slot
			putBits(inputs[offset:offset+bnLengthWords], x.Get(i).Bits(), bnLengthWords)
			offset += bnLengthWords
			// Put the second operand for this slot
			putBits(inputs[offset:offset+bnLengthWords], y.Get(i).Bits(), bnLengthWords)
			offset += bnLengthWords
		}
		if debugPrint {
			println("Call", callId, "post input arrangement", time.Since(start))
			start = time.Now()
		}

		// Upload, run, wait for download
		err := env.enqueue(stream, kernelMul2, int(numSlots))
		if debugPrint {
			println("Call", callId, "post put", time.Since(start))
			start = time.Now()
		}
		if err != nil {
			resultChan <- err
			return
		}

		outputs := stream.getCpuOutputsWords(env, kernelMul2, int(numSlots))

		// Wait on things to finish with Cuda
		_, err = get(stream)

		if debugPrint {
			println("Call", callId, "post get", time.Since(start))
			start = time.Now()
		}
		if err != nil {
			resultChan <- err
			return
		}

		// Everything is OK, so let's go ahead and import the results

		offset = 0
		for i := uint32(0); i < numSlots; i++ {
			// Output the computed result into each slot
			g.OverwriteBits(results.Get(i), outputs[offset:offset+bnLengthWords])
			offset += bnLengthWords
		}

		if debugPrint {
			println("Call", callId, "post output arrangement", time.Since(start))
		}

		resultChan <- nil
	}()
	return resultChan
}
