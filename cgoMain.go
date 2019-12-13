package main

/*
#cgo LDFLAGS: -Llib -lpowmosm75 -Wl,-rpath -Wl,./lib
#include "cgbnBindings/powm/powm_odd_export.h"
#include <stdlib.h>
#include <string.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"gitlab.com/elixxir/crypto/cyclic"
	"gitlab.com/elixxir/crypto/large"
	"unsafe"
)

// Package C enum in golang for testing, possible export?
const (
	kernelPowmOdd = C.KERNEL_POWM_ODD
	kernelElgamal = C.KERNEL_ELGAMAL
	kernelMul2    = C.KERNEL_MUL2
)

// Load the shared library and return any errors
// Copies a C string into a Go error and frees the C string
func GoError(cString *C.char) error {
	if cString != nil {
		errorStringGo := C.GoString(cString)
		err := errors.New(errorStringGo)
		C.free((unsafe.Pointer)(cString))
		return err
	}
	return nil
}

// Lay out powm4096 inputs in the correct order in a certain region of memory
// len(x) must be equal to len(y)
// For calculating x**y mod p
func prepare_powm_4096_inputs(x []*cyclic.Int, y []*cyclic.Int, inputMem []byte) {
	panic("Unimplemented")
}

// Should this take intbuffers and a range instead?
// What values do some of these (the ones that become results) actually have? Are they just 1 most of the time?
// Maybe a design that involves GPU is different enough from the CPU design which uses the same variable for input and output.
// Could ecrKeys and cypher just be outputs?
func stageElgamalInputs(privateKey, key, ecrKeys, cypher []*cyclic.Int) ([]byte, error) {
	if len(privateKey) != len(key) || len(privateKey) != len(ecrKeys) || len(privateKey) != len(cypher) {
		return nil, errors.New("lengths of all input arrays must be equal")
	}
	inputMem := make([]byte, 0, getInputsSizeElgamal(len(privateKey)))
	for i := 0; i < len(privateKey); i++ {
		inputMem = append(inputMem, privateKey[i].CGBNMem(bnSizeBits)...)
		inputMem = append(inputMem, key[i].CGBNMem(bnSizeBits)...)
		inputMem = append(inputMem, ecrKeys[i].CGBNMem(bnSizeBits)...)
		inputMem = append(inputMem, cypher[i].CGBNMem(bnSizeBits)...)
	}
	return inputMem, nil
}

func stageElgamalConstants(group *cyclic.Group, publicCypherKey *cyclic.Int) []byte {
	constantMem := make([]byte, 0, getConstantsSizeElgamal())
	constantMem = append(constantMem, group.GetG().CGBNMem(bnSizeBits)...)
	constantMem = append(constantMem, group.GetP().CGBNMem(bnSizeBits)...)
	constantMem = append(constantMem, publicCypherKey.CGBNMem(bnSizeBits)...)
	return constantMem
}

const (
	bnSizeBits  = 4096
	bnSizeBytes = bnSizeBits / 8
)

// Two numbers per input
func getInputsSizePowm4096(length int) int {
	return bnSizeBytes * 2 * length
}

// One number per output
func getOutputsSizePowm4096(length int) int {
	return bnSizeBytes * length
}

// One number (prime)
func getConstantsSizePowm4096() int {
	return bnSizeBytes
}

// Four numbers per input
func getInputsSizeElgamal(length int) int {
	return bnSizeBytes * 4 * length
}

// Two numbers per output
func getOutputsSizeElgamal(length int) int {
	return bnSizeBytes * 2 * length
}

// Three numbers (g, prime, publicCypherKey)
func getConstantsSizeElgamal() int {
	return bnSizeBytes * 3
}

// It would be nice to more easily pass the type of operation for creating the stream manager
// Returns pointer representing the stream manager
// If it's not magnificently inefficient, we could probably just create
// streams once and just not worry about lifetimes for them
// However, this will require having enough space for the inputs of all operations
func createStreams(numStreams int, capacity int, kernel int) ([]unsafe.Pointer, error) {
	streamCreateInfo := C.struct_streamCreateInfo{
		capacity: (C.size_t)(capacity),
	}
	// It might make more sense to create a stream with enough capacity to run all operations
	switch kernel {
	case kernelPowmOdd:
		streamCreateInfo.inputsCapacity = (C.size_t)(getInputsSizePowm4096(capacity))
		streamCreateInfo.outputsCapacity = (C.size_t)(getOutputsSizePowm4096(capacity))
		streamCreateInfo.constantsCapacity = (C.size_t)(getConstantsSizePowm4096())
	case kernelElgamal:
		streamCreateInfo.inputsCapacity = (C.size_t)(getInputsSizeElgamal(capacity))
		streamCreateInfo.outputsCapacity = (C.size_t)(getOutputsSizeElgamal(capacity))
		streamCreateInfo.constantsCapacity = (C.size_t)(getConstantsSizeElgamal())
	default:
		return nil, errors.New("unexpected kernel, don't know required sizes")
	}

	streams := make([]unsafe.Pointer, 0, numStreams)

	for i := 0; i < numStreams; i++ {
		createStreamResult := C.createStream(streamCreateInfo)
		stream := createStreamResult.result
		if stream != nil {
			streams = append(streams, stream)
		}
		if createStreamResult.error != nil {
			// Try to destroy all created streams to avoid leaking memory
			for j := 0; j < len(streams); j++ {
				C.destroyStream(streams[j])
			}
			return nil, GoError(createStreamResult.error)
		}
	}

	return streams, nil
}

func destroyStreams(streams []unsafe.Pointer) error {
	for i := 0; i < len(streams); i++ {
		err := C.destroyStream(streams[i])
		if err != nil {
			return GoError(err)
		}
	}
	return nil
}

// Calculate x**y mod p using CUDA
// Results are put in a byte array for translation back to cyclic ints elsewhere
// Currently, we upload and execute all in the same method

// Upload some items to the next stream
// Returns the stream that the data were uploaded to
// TODO Store the kernel enum for the upload in the stream
//  That way you don't have to pass that info again for run
//  There should be no scenario where the stream gets run for a different kernel than the upload
func upload(constantsMem []byte, inputMem []byte, length int, stream unsafe.Pointer, kernel int) error {
	// get pointers to pinned memory
	inputs := C.getCpuInputs(stream)
	constants := C.getCpuConstants(stream)
	// copy to pinned memory
	// I assume that a normal golang copy() call wouldn't work,
	// because they aren't both slices
	// TODO Bounds check this at least? The whole lengths situation isn't good
	C.memcpy(inputs, (unsafe.Pointer)(&inputMem[0]), (C.size_t)(len(inputMem)))
	C.memcpy(constants, (unsafe.Pointer)(&constantsMem[0]), (C.size_t)(len(constantsMem)))
	// queue upload
	var outputSize int
	switch kernel {
	case kernelPowmOdd:
		outputSize = getOutputsSizePowm4096(length)
	case kernelElgamal:
		outputSize = getOutputsSizeElgamal(length)
	}
	uploadError := C.upload((C.uint)(length), stream, (C.size_t)(len(inputMem)), (C.size_t)(len(constantsMem)), (C.size_t)(outputSize))
	if uploadError != nil {
		return GoError(uploadError)
	} else {
		return nil
	}
}

// Can you use the C type like this?
// Might need to redefine enumeration in Golang
func run(stream unsafe.Pointer, whichToRun C.enum_kernel) error {
	return GoError(C.run(stream, whichToRun))
}

// Enqueue a download for this stream after execution finishes
// Doesn't actually block for the download
func download(stream unsafe.Pointer) error {
	return GoError(C.download(stream))
}

// Wait for this stream's download to finish and return a pointer to the results
// This also checks the CGBN error report (presumably this is where things should be checked, if not now, then in the future, to see whether they're in the group or not. However this may not(?) be doable if everything is in Montgomery space.)
func getResults(stream unsafe.Pointer, numOutputs int, kernel int) ([]byte, error) {
	result := C.getResults(stream)
	// Only need to free the result, not the underlying pointers
	// result.result is a long-lived pinned memory buffer, and it doesn't need to be freed
	defer C.free(unsafe.Pointer(result))
	var resultBytes []byte
	switch kernel {
	case kernelPowmOdd:
		resultBytes = C.GoBytes(result.result, (C.int)(getOutputsSizePowm4096(numOutputs)))
	case kernelElgamal:
		resultBytes = C.GoBytes(result.result, (C.int)(getOutputsSizeElgamal(numOutputs)))
	}
	resultError := GoError(result.error)
	return resultBytes, resultError
}

// Deprecated. Use the decomposed methods instead
func powm4096(primeMem []byte, inputMem []byte, length int) ([]byte, error) {
	streams, err := createStreams(1, length, kernelPowmOdd)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := destroyStreams(streams)
		if err != nil {
			panic(err)
		}
	}()
	stream := streams[0]
	err = upload(primeMem, inputMem, length, stream, kernelPowmOdd)
	if err != nil {
		return nil, err
	}
	err = run(stream, C.KERNEL_POWM_ODD)
	if err != nil {
		return nil, err
	}
	err = download(stream)
	if err != nil {
		return nil, err
	}
	return getResults(stream, length, kernelPowmOdd)
}

// Start GPU profiling
// You need to call this if you're starting and stopping profiling all willy-nilly,
// like for a benchmark
func startProfiling() error {
	errString := C.startProfiling()
	err := GoError(errString)
	return err
}

// Stop GPU profiling
func stopProfiling() error {
	errString := C.stopProfiling()
	err := GoError(errString)
	return err
}

// Reset the CUDA device
// Hopefully this will allow the CUDA profile to be gotten in the graphical profiler
func resetDevice() error {
	errString := C.resetDevice()
	err := GoError(errString)
	return err
}

func main() {
	// Not sure what q would be for MODP4096, so leaving it at 1
	g := cyclic.NewGroup(
		large.NewIntFromString("FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3DC2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F83655D23DCA3AD961C62F356208552BB9ED529077096966D670C354E4ABC9804F1746C08CA18217C32905E462E36CE3BE39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9DE2BCBF6955817183995497CEA956AE515D2261898FA051015728E5A8AACAA68FFFFFFFFFFFFFFFF", 16),
		large.NewInt(2),
	)
	// x**y mod p
	x := g.NewIntFromString("102698389601429893247415098320984", 10)
	y := g.NewIntFromString("8891261048623689650221543816983486", 10)
	pMem := g.GetP().CGBNMem(bnSizeBits)
	result := g.Exp(x, y, g.NewInt(2))
	fmt.Printf("result in Go: %v\n", result.TextVerbose(16, 0))
	// x**y mod p: x (4096 bits)
	// For more than one X and Y, they would be appended in the list
	var cgbnInputs []byte
	cgbnInputs = append(cgbnInputs, x.CGBNMem(bnSizeBits)...)
	cgbnInputs = append(cgbnInputs, y.CGBNMem(bnSizeBits)...)
	inputsMem := cgbnInputs
	resultBytes, err := powm4096(pMem, inputsMem, 1)
	if err != nil {
		panic(err)
	}
	resultInt := g.NewIntFromCGBN(resultBytes[:bnSizeBytes])
	fmt.Printf("result in Go from CUDA: %v\n", resultInt.TextVerbose(16, 0))
	err = stopProfiling()
	if err != nil {
		panic(err)
	}
	err = resetDevice()
	if err != nil {
		panic(err)
	}
}
