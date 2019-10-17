package main

/*
#cgo LDFLAGS: -Llib -lpowmosm75 -Wl,-rpath -Wl,./lib
#include "cgbnBindings/powm/powm_odd_export.h"
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"gitlab.com/elixxir/crypto/cyclic"
	"gitlab.com/elixxir/crypto/large"
	"unsafe"
)

const bitLen = 4096

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

func freeResult(result *C.struct_return_data) {
	if result != nil {
		if result.result != nil {
			C.free((unsafe.Pointer)(result.result))
		}
		if result.error != nil {
			C.free(unsafe.Pointer(result.error))
		}
		C.free((unsafe.Pointer)(result))
	}
}

func getInputsSizePowm4096(length int) int {
	return int(C.getInputsSize_powm4096((C.size_t)(length)))
}

func getOutputsSizePowm4096(length int) int {
	return int(C.getOutputsSize_powm4096((C.size_t)(length)))
}

func getConstantsSizePowm4096() int {
	return int(C.getConstantsSize_powm4096())
}

// It would be nice to more easily pass the type of operation for creating the stream manager
// Returns pointer representing the stream manager
func createStreamManagerPowm4096(numStreams int, capacity int) (unsafe.Pointer, error) {
	streamManagerCreateInfo := C.struct_streamManagerCreateInfo{
		numStreams:    (C.size_t)(numStreams),
		capacity:      (C.size_t)(capacity),
		inputsSize:    (C.size_t)(getInputsSizePowm4096(capacity)),
		outputsSize:   (C.size_t)(getOutputsSizePowm4096(capacity)),
		constantsSize: (C.size_t)(getConstantsSizePowm4096()),
	}
	createStreamManagerResult := C.createStreamManager(streamManagerCreateInfo)
	defer C.free((unsafe.Pointer)(createStreamManagerResult))
	if createStreamManagerResult.error != nil {
		// nanc Can destroyStreamManager properly handle the case where streamManager was created partially due to an error?
		// This could actually be incorrect behavior
		defer C.destroyStreamManager(createStreamManagerResult.result)
		return nil, GoError(createStreamManagerResult.error)
	}
	return createStreamManagerResult.result, nil
}

func destroyStreamManager(streamManager unsafe.Pointer) error {
	err := C.destroyStreamManager(streamManager)
	if err != nil {
		return GoError(err)
	}
	return nil
}

// Calculate x**y mod p using CUDA
// Results are put in a byte array for translation back to cyclic ints elsewhere
// Currently, we upload and execute all in the same method

// Upload some items to the next stream
// Returns the stream that the data were uploaded to
func uploadPowm4096(primeMem []byte, inputMem []byte, length int, streamManager unsafe.Pointer) (unsafe.Pointer, error) {
	stream := C.getNextStream(streamManager)
	uploadError := C.upload_powm_4096(C.CBytes(primeMem), C.CBytes(inputMem), (C.uint)(length), stream)
	if uploadError != nil {
		return nil, GoError(uploadError)
	} else {
		return stream, nil
	}
}

func runPowm4096(stream unsafe.Pointer) error {
	return GoError(C.run_powm_4096(stream))
}

func downloadPowm4096(stream unsafe.Pointer, numOutputs int) ([]byte, error) {
	downloadResult := C.download_powm_4096(stream)
	defer freeResult(downloadResult)
	return C.GoBytes(downloadResult.result, C.int(getOutputsSizePowm4096(numOutputs))), GoError(downloadResult.error)
}

// Deprecated. Use the decomposed methods instead
func powm4096(primeMem []byte, inputMem []byte, length int) ([]byte, error) {
	streamManager, err := createStreamManagerPowm4096(1, length)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := destroyStreamManager(streamManager)
		if err != nil {
			panic(err)
		}
	}()
	stream, err := uploadPowm4096(primeMem, inputMem, length, streamManager)
	if err != nil {
		return nil, err
	}
	err = runPowm4096(stream)
	if err != nil {
		return nil, err
	}
	return downloadPowm4096(stream, length)
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
		large.NewInt(1),
	)
	// x**y mod p
	x := g.NewIntFromString("102698389601429893247415098320984", 10)
	y := g.NewIntFromString("8891261048623689650221543816983486", 10)
	pMem := g.GetP().CGBNMem(bitLen)
	result := g.Exp(x, y, g.NewInt(2))
	fmt.Printf("result in Go: %v\n", result.TextVerbose(16, 0))
	// x**y mod p: x (4096 bits)
	// For more than one X and Y, they would be appended in the list
	var cgbnInputs []byte
	cgbnInputs = append(cgbnInputs, x.CGBNMem(bitLen)...)
	cgbnInputs = append(cgbnInputs, y.CGBNMem(bitLen)...)
	inputsMem := cgbnInputs
	resultBytes, err := powm4096(pMem, inputsMem, 1)
	if err != nil {
		panic(err)
	}
	resultInt := g.NewIntFromCGBN(resultBytes[:bitLen/8])
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
