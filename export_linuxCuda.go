//+build linux,cuda

package gpumaths

/*#cgo LDFLAGS: -Llib -lpowmosm75 -Wl,-rpath -Wl,./lib
#include "cgbnBindings/powm/powm_odd_export.h"
*/
import "C"
import (
	"fmt"
	"log"
	"reflect"
	"unsafe"
)

const (
	bnLength = 4096
	bnLengthBytes = bnLength/8
)

// publicCypherKey and prime should be byte slices obtained by running .Bytes() on the large int
// The resulting byte slice should be trimmed and should be less than or equal to the length of
// the template-instantiated BN on the GPU.
// bnLength is a length in bits
// TODO validate BN length in code (i.e. pick kernel variants based on bn length)
func ElGamal(input ElGamalInput, stream Stream) chan ElGamalResult {
	// Return the result later, when the GPU job finishes
	resultChan := make(chan ElGamalResult, 1)
	go func() {
		validateElgamalInput(input, stream)

		// Arrange memory into stream buffers
		numSlots := len(input.Slots)

		// TODO clean this up by implementing the arrangement/dearrangement with reader/writer interfaces
		//  or smth
		constants := toSlice(C.getCpuConstants(stream.s), int(C.getConstantsSize(kernelElgamal)))
		offset := 0
		putInt(constants[offset:offset+bnLengthBytes], input.G, bnLengthBytes)
		offset += bnLengthBytes
		putInt(constants[offset:offset+bnLengthBytes], input.Prime, bnLengthBytes)
		offset += bnLengthBytes
		putInt(constants[offset:offset+bnLengthBytes], input.PublicCypherKey, bnLengthBytes)

		inputs := toSlice(C.getCpuInputs(stream.s, kernelElgamal), int(C.getInputSize(kernelElgamal))*numSlots)
		offset = 0
		for i := 0; i < numSlots; i++ {
			putInt(inputs[offset:offset+bnLengthBytes], input.Slots[i].PrivateKey, bnLengthBytes)
			offset += bnLengthBytes
			putInt(inputs[offset:offset+bnLengthBytes], input.Slots[i].Key, bnLengthBytes)
			offset += bnLengthBytes
			putInt(inputs[offset:offset+bnLengthBytes], input.Slots[i].EcrKey, bnLengthBytes)
			offset += bnLengthBytes
			putInt(inputs[offset:offset+bnLengthBytes], input.Slots[i].Cypher, bnLengthBytes)
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
		resultBuf := make([]byte, C.getOutputSize(kernelElgamal)*(C.size_t)(numSlots))
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
			result.Slots[i].EcrKey = resultBuf[offset : offset+bnLengthBytes]
			putInt(result.Slots[i].EcrKey, results[offset:offset+bnLengthBytes], bnLengthBytes)
			offset += bnLengthBytes
			result.Slots[i].Cypher = resultBuf[offset : offset+bnLengthBytes]
			putInt(result.Slots[i].Cypher, results[offset:offset+bnLengthBytes], bnLengthBytes)
			offset += bnLengthBytes
		}

		resultChan <- result
	}()
	return resultChan
}

// Precondition: Modulus must be odd
func Exp(input ExpInput, stream Stream) chan ExpResult {
	// Return the result later, when the GPU job finishes
	resultChan := make(chan ExpResult, 1)
	go func() {
		validateExpInput(input, stream)

		// Arrange memory into stream buffers
		numSlots := len(input.Slots)

		// TODO clean this up by implementing the arrangement/dearrangement with reader/writer interfaces
		//  or smth
		constants := toSlice(C.getCpuConstants(stream.s), int(C.getConstantsSize(kernelPowmOdd)))
		offset := 0
		putInt(constants[offset:offset+bnLengthBytes], input.Modulus, bnLengthBytes)

		inputs := toSlice(C.getCpuInputs(stream.s, kernelPowmOdd), int(C.getInputSize(kernelPowmOdd))*numSlots)
		offset = 0
		for i := 0; i < numSlots; i++ {
			putInt(inputs[offset:offset+bnLengthBytes], input.Slots[i].Base, bnLengthBytes)
			offset += bnLengthBytes
			putInt(inputs[offset:offset+bnLengthBytes], input.Slots[i].Exponent, bnLengthBytes)
			offset += bnLengthBytes
		}

		// Upload, run, wait for download
		err := put(stream, kernelPowmOdd, numSlots)
		if err != nil {
			resultChan <- ExpResult{Err: err}
			return
		}
		err = run(stream)
		if err != nil {
			resultChan <- ExpResult{Err: err}
			return
		}
		err = download(stream)
		if err != nil {
			resultChan <- ExpResult{Err: err}
			return
		}

		// Results will be stored in this buffer
		resultBuf := make([]byte, C.getOutputSize(kernelPowmOdd)*(C.size_t)(numSlots))
		results := toSlice(C.getCpuOutputs(stream.s), len(resultBuf))

		// Wait on things to finish with Cuda
		err = get(stream)
		if err != nil {
			resultChan <- ExpResult{Err: err}
			return
		}

		// Everything is OK, so let's go ahead and import the results
		result := ExpResult{
			Results: make([][]byte, numSlots),
			Err:   nil,
		}

		offset = 0
		for i := 0; i < numSlots; i++ {
			result.Results[i]= resultBuf[offset : offset+bnLengthBytes]
			putInt(result.Results[i], results[offset:offset+bnLengthBytes], bnLengthBytes)
			offset += bnLengthBytes
		}

		resultChan <- result
	}()
	return resultChan
}

// Bounds check to make sure that the stream can take all the inputs
func validateElgamalInput(input ElGamalInput, stream Stream) {
	if len(input.Slots) > stream.maxSlotsElGamal {
		// This can only happen because of user error (unlike Cuda problems), so panic to make the error apparent
		log.Panicf(fmt.Sprintf("%v slots is more than this stream's max of %v for ElGamal kernel", len(input.Slots), stream.maxSlotsElGamal))
	}
}

// Checks if there's a number of exp slots that the stream can handle
func validateExpInput(input ExpInput, stream Stream) {
	if len(input.Slots) > stream.maxSlotsExp {
		log.Panicf(fmt.Sprintf("%v slots is more than this stream's max of %v for Exp kernel", len(input.Slots), stream.maxSlotsExp))
	}
}

// Create byte slice viewing memory at a certain memory address with a certain length
// Here be dragons
func toSlice(pointer unsafe.Pointer, size int) []byte {
	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{Data: uintptr(pointer), Len: size, Cap: size}))
}
