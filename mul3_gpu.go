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

const kernelMul3 = C.KERNEL_MUL3

// Mul3Chunk performs the mul3 operation on the cypher and precomputation
// payloads
// Precondition: All int buffers must have the same length
var Mul3Chunk Mul3ChunkPrototype = func(p *StreamPool, g *cyclic.Group,
	x *cyclic.IntBuffer, y *cyclic.IntBuffer, z *cyclic.IntBuffer, results *cyclic.IntBuffer) error {
	// Populate mul3 inputs
	numSlots := uint32(x.Len())

	// Run kernel on the inputs
	stream := p.TakeStream()
	defer p.ReturnStream(stream)
	env := chooseEnv(g)
	maxSlotsMul3 := uint32(env.maxSlots(len(stream.cpuData), kernelMul3))
	for i := uint32(0); i < numSlots; i += maxSlotsMul3 {
		sliceEnd := i
		// Don't slice beyond the end of the input slice
		if i+maxSlotsMul3 <= numSlots {
			sliceEnd += maxSlotsMul3
		} else {
			sliceEnd = numSlots
		}
		err := <-mul3(g, x.GetSubBuffer(i, sliceEnd), y.GetSubBuffer(i, sliceEnd), z.GetSubBuffer(i, sliceEnd), results.GetSubBuffer(i, sliceEnd), env, stream)
		if err != nil {
			return err
		}
	}

	return nil
}

func mul3(g *cyclic.Group, x *cyclic.IntBuffer, y *cyclic.IntBuffer, z *cyclic.IntBuffer, result *cyclic.IntBuffer, env gpumathsEnv, stream Stream) chan error {
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
		constants := stream.getCpuConstantsWords(env, kernelMul3)
		bnLengthWords := env.getWordLen()
		putBits(constants, g.GetP().Bits(), bnLengthWords)
		offset := 0
		offset += bnLengthWords

		inputs := stream.getCpuInputsWords(env, kernelMul3, int(numSlots))
		offset = 0
		for i := uint32(0); i < numSlots; i++ {
			// Put the first operand for this slot
			putBits(inputs[offset:offset+bnLengthWords], x.Get(i).Bits(), bnLengthWords)
			offset += bnLengthWords
			// Put the second operand for this slot
			putBits(inputs[offset:offset+bnLengthWords], y.Get(i).Bits(), bnLengthWords)
			offset += bnLengthWords
			// Put the third operand for this slot
			putBits(inputs[offset:offset+bnLengthWords], z.Get(i).Bits(), bnLengthWords)
			offset += bnLengthWords
		}
		if debugPrint {
			println("Call", callId, "post input arrangement", time.Since(start))
			start = time.Now()
		}

		// Upload, run, wait for download
		err := env.enqueue(stream, kernelMul3, int(numSlots))
		if debugPrint {
			println("Call", callId, "post put", time.Since(start))
			start = time.Now()
		}
		if err != nil {
			resultChan <- err
			return
		}

		outputs := stream.getCpuOutputsWords(env, kernelMul3, int(numSlots))

		// Wait on things to finish with Cuda
		err = get(stream)

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
			g.OverwriteBits(result.Get(i), outputs[offset:offset+bnLengthWords])
			offset += bnLengthWords
		}

		if debugPrint {
			println("Call", callId, "post output arrangement", time.Since(start))
		}

		resultChan <- nil
	}()
	return resultChan
}
