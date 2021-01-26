///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

//+build linux,gpu

package gpumaths

// gpu.go contains helper functions and constants used by
// the gpu implementation. See the exp, elgamal, reveal, or strip _gpu.go
// files for implementations of specific operations.

// When the gpumaths library itself is under development, it should
// use the version of gpumaths that's built in-repository
// (./lib/libpowmosm75.so). golang puts a ./lib entry in the rpath
// itself (as far as I can tell, but it's after everything, so
// to have the ./lib version take priority, there's another entry
// before so the development version takes precedence if both are
// present

/*
#cgo CFLAGS: -I./cgbnBindings/powm -I/opt/xxnetwork/include
#cgo LDFLAGS: -L/opt/xxnetwork/lib -lpowmosm75 -Wl,-rpath,./lib:/opt/xxnetwork/lib
#include <powm_odd_export.h>
#include <stdlib.h>
#include <string.h>
*/
import "C"
import (
	"fmt"
	"github.com/pkg/errors"
	"gitlab.com/elixxir/crypto/cyclic"
	"gitlab.com/xx_network/crypto/large"
	"math/big"
	"reflect"
	"sync"
	"unsafe"
)

type gpumathsEnv interface {
	// enqueue calls put, run, and download all together
	enqueue(stream Stream, whichToRun C.enum_kernel, numSlots int) error
	getBitLen() int
	getByteLen() int
	getWordLen() int
	getConstantsSize(C.enum_kernel) int
	getOutputSize(C.enum_kernel) int
	getInputSize(C.enum_kernel) int
	// Get the number of words (in large.Bits type) that the constants for this
	// kernel take up
	getConstantsSizeWords(C.enum_kernel) int
	getOutputSizeWords(C.enum_kernel) int
	getInputSizeWords(C.enum_kernel) int
	maxSlots(memSize int, op C.enum_kernel) int
	streamSizeContaining(numItems int, kernel int) int
}

// TODO These types implement gpumaths? interface
type (
	gpumaths2048 struct{ sizeData }
	gpumaths3200 struct{ sizeData }
	gpumaths4096 struct{ sizeData }
)

var gpumathsEnv2048 gpumaths2048
var gpumathsEnv3200 gpumaths3200
var gpumathsEnv4096 gpumaths4096
var cudaDone sync.Once

// All size data that a gpumath env could get is included in this type
// Since these calls will always have the same result,
// there's no need for synchronization mechanisms when using this data structure
type sizeData [C.NUM_KERNELS]struct {
	inputSize          int
	constantsSize      int
	outputSize         int
	inputSizeWords     int
	constantsSizeWords int
	outputSizeWords    int
}

// Should the envs belong to the stream pool? probably not
func chooseEnv(g *cyclic.Group) gpumathsEnv {
	primeLen := g.GetP().BitLen()
	len2048 := gpumathsEnv2048.getBitLen()
	len3200 := gpumathsEnv3200.getBitLen()
	len4096 := gpumathsEnv4096.getBitLen()
	if primeLen <= len2048 {
		return &gpumathsEnv2048
	} else if primeLen <= len3200 {
		return &gpumathsEnv3200
	} else if primeLen <= len4096 {
		return &gpumathsEnv4096
	} else {
		panic(fmt.Sprintf("Prime %s was too big for any available gpumaths environment", g.GetP().Text(16)))
	}
}

func (gpumaths2048) getBitLen() int {
	return 2048
}
func (gpumaths2048) getByteLen() int {
	return 2048 / 8
}
func (g gpumaths2048) getWordLen() int {
	// TODO large.Word?
	return g.getByteLen() / int(unsafe.Sizeof(big.Word(0)))
}
func (gpumaths3200) getBitLen() int {
	return 3200
}
func (gpumaths3200) getByteLen() int {
	return 3200 / 8
}
func (g gpumaths3200) getWordLen() int {
	// TODO large.Word?
	return g.getByteLen() / int(unsafe.Sizeof(big.Word(0)))
}
func (gpumaths4096) getBitLen() int {
	return 4096
}
func (gpumaths4096) getByteLen() int {
	return 4096 / 8
}
func (g gpumaths4096) getWordLen() int {
	// TODO large.Word?
	return g.getByteLen() / int(unsafe.Sizeof(big.Word(0)))
}

// Create byte slice viewing memory at a certain memory address with a
// certain length
// Here be dragons
func toSlice(pointer unsafe.Pointer, size int) []byte {
	return *(*[]byte)(unsafe.Pointer(
		&reflect.SliceHeader{Data: uintptr(pointer),
			Len: size, Cap: size}))
}

func toSliceOfWords(pointer unsafe.Pointer, size int) large.Bits {
	return *(*large.Bits)(unsafe.Pointer(
		&reflect.SliceHeader{Data: uintptr(pointer),
			Len: size, Cap: size}))
}

// Load the shared library and return any errors
// Copies a C string into a Go error and frees the C string
func goError(cString *C.char) error {
	if cString != nil {
		errorStringGo := C.GoString(cString)
		err := errors.New(errorStringGo)
		C.free((unsafe.Pointer)(cString))
		return err
	}
	return nil
}

// Creates streams of a particular size meant to run a particular operation
func createStreams(numStreams int, capacity int) ([]Stream, error) {
	streamCreateInfo := C.struct_streamCreateInfo{
		capacity: C.size_t(capacity),
	}

	streams := make([]Stream, 0, numStreams)

	for i := 0; i < numStreams; i++ {
		// We need to free this createStreamResult, right?
		// Or, it might be possible to return the struct by value instead.
		createStreamResult := C.createStream(streamCreateInfo)

		// TODO Any possibility of double free here in error cases?
		// Check for normally created error first, if it exists
		if createStreamResult != nil && createStreamResult.error != nil {
			createError := goError(createStreamResult.error)
			// Attempt to clean up any streams that were successfully created
			destroyErr := destroyStreams(append(streams, Stream{s: createStreamResult.result}))
			C.free(unsafe.Pointer(createStreamResult))
			if destroyErr != nil && createError != nil {
				return nil, errors.Wrap(destroyErr, createError.Error())
			} else if createError != nil {
				return nil, createError
			}
		} else if createStreamResult != nil && C.isStreamValid(createStreamResult.result) == 0 {
			// No error, but something in the stream wasn't set
			// Attempt to clean up any streams that were successfully created
			destroyErr := destroyStreams(append(streams, Stream{s: createStreamResult.result}))
			C.free(unsafe.Pointer(createStreamResult))
			if destroyErr != nil {
				return nil, errors.Wrap(destroyErr, "not all fields of stream were initialized")
			} else {
				return nil, errors.New("not all fields of stream were initialized")
			}
		} else if createStreamResult == nil {
			// Unlikely error, but one of the allocations for createStream return structures must have failed
			// Attempt to clean up any streams that were successfully created
			destroyError := destroyStreams(streams)
			return nil, destroyError
		}

		// If we got here, we should have a good stream result from createStream
		if createStreamResult.result != nil && createStreamResult.cpuBuf != nil {
			sizeofOperand := make(large.Bits, 1)
			streams = append(streams, Stream{
				s:            createStreamResult.result,
				cpuData:      toSlice(createStreamResult.cpuBuf, capacity),
				cpuDataWords: toSliceOfWords(createStreamResult.cpuBuf, int(uintptr(capacity)/unsafe.Sizeof(sizeofOperand[0]))),
			})
		}
		// Double free possible here?
		C.free(unsafe.Pointer(createStreamResult))
	}

	return streams, nil
}

func destroyStreams(streams []Stream) error {
	for i := 0; i < len(streams); i++ {
		err := C.destroyStream(streams[i].s)
		if err != nil {
			return goError(err)
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
// Could return byte slices of output as well? perhaps?
func (gpumaths2048) enqueue(stream Stream, whichToRun C.enum_kernel, numSlots int) error {
	//return errors.New("temporarily disabled due to driver API migration")
	uploadError := C.enqueue2048(C.uint(numSlots), stream.s, whichToRun)
	if uploadError != nil {
		return goError(uploadError)
	} else {
		return nil
	}
}
func (gpumaths3200) enqueue(stream Stream, whichToRun C.enum_kernel, numSlots int) error {
	//return errors.New("temporarily disabled due to driver API migration")
	uploadError := C.enqueue3200(C.uint(numSlots), stream.s, whichToRun)
	if uploadError != nil {
		return goError(uploadError)
	} else {
		return nil
	}
}
func (gpumaths4096) enqueue(stream Stream, whichToRun C.enum_kernel, numSlots int) error {
	uploadError := C.enqueue4096(C.uint(numSlots), stream.s, whichToRun)
	if uploadError != nil {
		return goError(uploadError)
	} else {
		return nil
	}
}

// Populate the sizes of constants, inputs, outputs in words based on the byte sizes
func (s *sizeData) populateWordSizes(kernel C.enum_kernel) {
	sizeOfOperand := make(large.Bits, 1)
	sizeOfWord := int(unsafe.Sizeof(sizeOfOperand[0]))
	s[kernel].inputSizeWords = s[kernel].inputSize / sizeOfWord
	s[kernel].constantsSizeWords = s[kernel].constantsSize / sizeOfWord
	s[kernel].outputSizeWords = s[kernel].outputSize / sizeOfWord
}

func (g *gpumaths2048) populateSizeData(kernel C.enum_kernel) {
	g.sizeData[kernel].inputSize = int(C.getInputSize2048(kernel))
	// If the result is zero, the kernel is unknown
	// These panics should never happen unless there's programmer error
	if g.sizeData[kernel].inputSize == 0 {
		panic(fmt.Sprintf("Couldn't find input size for kernel %v", kernel))
	}
	g.sizeData[kernel].outputSize = int(C.getOutputSize2048(kernel))
	if g.sizeData[kernel].outputSize == 0 {
		panic(fmt.Sprintf("Couldn't find output size for kernel %v", kernel))
	}
	g.sizeData[kernel].constantsSize = int(C.getConstantsSize2048(kernel))
	if g.sizeData[kernel].constantsSize == 0 {
		panic(fmt.Sprintf("Couldn't find constants size for kernel %v", kernel))
	}
	g.sizeData.populateWordSizes(kernel)
}
func (g *gpumaths3200) populateSizeData(kernel C.enum_kernel) {
	g.sizeData[kernel].inputSize = int(C.getInputSize3200(kernel))
	// If the result is zero, the kernel is unknown
	// These panics should never happen unless there's programmer error
	if g.sizeData[kernel].inputSize == 0 {
		panic(fmt.Sprintf("Couldn't find input size for kernel %v", kernel))
	}
	g.sizeData[kernel].outputSize = int(C.getOutputSize3200(kernel))
	if g.sizeData[kernel].outputSize == 0 {
		panic(fmt.Sprintf("Couldn't find output size for kernel %v", kernel))
	}
	g.sizeData[kernel].constantsSize = int(C.getConstantsSize3200(kernel))
	if g.sizeData[kernel].constantsSize == 0 {
		panic(fmt.Sprintf("Couldn't find constants size for kernel %v", kernel))
	}
	g.sizeData.populateWordSizes(kernel)
}
func (g *gpumaths4096) populateSizeData(kernel C.enum_kernel) {
	g.sizeData[kernel].inputSize = int(C.getInputSize4096(kernel))
	// If the result is zero, the kernel is unknown
	// These panics should never happen unless there's programmer error
	if g.sizeData[kernel].inputSize == 0 {
		panic(fmt.Sprintf("Couldn't find input size for kernel %v", kernel))
	}
	g.sizeData[kernel].outputSize = int(C.getOutputSize4096(kernel))
	if g.sizeData[kernel].outputSize == 0 {
		panic(fmt.Sprintf("Couldn't find output size for kernel %v", kernel))
	}
	g.sizeData[kernel].constantsSize = int(C.getConstantsSize4096(kernel))
	if g.sizeData[kernel].constantsSize == 0 {
		panic(fmt.Sprintf("Couldn't find constants size for kernel %v", kernel))
	}
	g.sizeData.populateWordSizes(kernel)
}

// Four numbers per input
// Returns size in bytes
func (g *gpumaths2048) getInputSize(kernel C.enum_kernel) int {
	if g.sizeData[kernel].inputSize == 0 {
		g.populateSizeData(kernel)
	}
	return g.sizeData[kernel].inputSize
}
func (g *gpumaths3200) getInputSize(kernel C.enum_kernel) int {
	if g.sizeData[kernel].inputSize == 0 {
		g.populateSizeData(kernel)
	}
	return g.sizeData[kernel].inputSize
}
func (g *gpumaths4096) getInputSize(kernel C.enum_kernel) int {
	if g.sizeData[kernel].inputSize == 0 {
		g.populateSizeData(kernel)
	}
	return g.sizeData[kernel].inputSize
}

// Returns size in words
func (g *gpumaths2048) getInputSizeWords(kernel C.enum_kernel) int {
	if g.sizeData[kernel].inputSizeWords == 0 {
		g.populateSizeData(kernel)
	}
	return g.sizeData[kernel].inputSizeWords
}
func (g *gpumaths3200) getInputSizeWords(kernel C.enum_kernel) int {
	if g.sizeData[kernel].inputSizeWords == 0 {
		g.populateSizeData(kernel)
	}
	return g.sizeData[kernel].inputSizeWords
}

// Might be able to refactor this for less repetition...
func (g *gpumaths4096) getInputSizeWords(kernel C.enum_kernel) int {
	if g.sizeData[kernel].inputSizeWords == 0 {
		g.populateSizeData(kernel)
	}
	return g.sizeData[kernel].inputSizeWords
}

// Returns size in bytes
func (g *gpumaths2048) getOutputSize(kernel C.enum_kernel) int {
	if g.sizeData[kernel].outputSize == 0 {
		g.populateSizeData(kernel)
	}
	return g.sizeData[kernel].outputSize
}
func (g *gpumaths3200) getOutputSize(kernel C.enum_kernel) int {
	if g.sizeData[kernel].outputSize == 0 {
		g.populateSizeData(kernel)
	}
	return g.sizeData[kernel].outputSize
}
func (g *gpumaths4096) getOutputSize(kernel C.enum_kernel) int {
	if g.sizeData[kernel].outputSize == 0 {
		g.populateSizeData(kernel)
	}
	return g.sizeData[kernel].outputSize
}

// Returns size in words
func (g *gpumaths2048) getOutputSizeWords(kernel C.enum_kernel) int {
	if g.sizeData[kernel].outputSizeWords == 0 {
		g.populateSizeData(kernel)
	}
	return g.sizeData[kernel].outputSizeWords
}
func (g *gpumaths3200) getOutputSizeWords(kernel C.enum_kernel) int {
	if g.sizeData[kernel].outputSizeWords == 0 {
		g.populateSizeData(kernel)
	}
	return g.sizeData[kernel].outputSizeWords
}
func (g *gpumaths4096) getOutputSizeWords(kernel C.enum_kernel) int {
	if g.sizeData[kernel].outputSizeWords == 0 {
		g.populateSizeData(kernel)
	}
	return g.sizeData[kernel].outputSizeWords
}

// Returns size in bytes
func (g *gpumaths2048) getConstantsSize(kernel C.enum_kernel) int {
	if g.sizeData[kernel].constantsSize == 0 {
		g.populateSizeData(kernel)
	}
	return g.sizeData[kernel].constantsSize
}
func (g *gpumaths3200) getConstantsSize(kernel C.enum_kernel) int {
	if g.sizeData[kernel].constantsSize == 0 {
		g.populateSizeData(kernel)
	}
	return g.sizeData[kernel].constantsSize
}
func (g *gpumaths4096) getConstantsSize(kernel C.enum_kernel) int {
	if g.sizeData[kernel].constantsSize == 0 {
		g.populateSizeData(kernel)
	}
	return g.sizeData[kernel].constantsSize
}
func (g *gpumaths2048) getConstantsSizeWords(kernel C.enum_kernel) int {
	if g.sizeData[kernel].constantsSizeWords == 0 {
		g.populateSizeData(kernel)
	}
	return g.sizeData[kernel].constantsSizeWords
}
func (g *gpumaths3200) getConstantsSizeWords(kernel C.enum_kernel) int {
	if g.sizeData[kernel].constantsSizeWords == 0 {
		g.populateSizeData(kernel)
	}
	return g.sizeData[kernel].constantsSizeWords
}
func (g *gpumaths4096) getConstantsSizeWords(kernel C.enum_kernel) int {
	if g.sizeData[kernel].constantsSizeWords == 0 {
		g.populateSizeData(kernel)
	}
	return g.sizeData[kernel].constantsSizeWords
}

// Helper functions for sizing
// Get the number of slots for an operation
func (g *gpumaths2048) maxSlots(memSize int, op C.enum_kernel) int {
	constantsSize := g.getConstantsSize(op)
	slotSize := g.getInputSize(op) + g.getOutputSize(op)
	memForSlots := memSize - constantsSize
	if memForSlots < 0 {
		return 0
	} else {
		return memForSlots / slotSize
	}
}

func (g *gpumaths3200) maxSlots(memSize int, op C.enum_kernel) int {
	constantsSize := g.getConstantsSize(op)
	slotSize := g.getInputSize(op) + g.getOutputSize(op)
	memForSlots := memSize - constantsSize
	if memForSlots < 0 {
		return 0
	} else {
		return memForSlots / slotSize
	}
}

func (g *gpumaths4096) maxSlots(memSize int, op C.enum_kernel) int {
	constantsSize := g.getConstantsSize(op)
	slotSize := g.getInputSize(op) + g.getOutputSize(op)
	memForSlots := memSize - constantsSize
	if memForSlots < 0 {
		return 0
	} else {
		return memForSlots / slotSize
	}
}

func (g *gpumaths2048) streamSizeContaining(numItems int, kernel int) int {
	return g.getInputSize(C.enum_kernel(kernel))*numItems +
		g.getOutputSize(C.enum_kernel(kernel))*numItems +
		g.getConstantsSize(C.enum_kernel(kernel))
}

func (g *gpumaths3200) streamSizeContaining(numItems int, kernel int) int {
	return g.getInputSize(C.enum_kernel(kernel))*numItems +
		g.getOutputSize(C.enum_kernel(kernel))*numItems +
		g.getConstantsSize(C.enum_kernel(kernel))
}

func (g *gpumaths4096) streamSizeContaining(numItems int, kernel int) int {
	return g.getInputSize(C.enum_kernel(kernel))*numItems +
		g.getOutputSize(C.enum_kernel(kernel))*numItems +
		g.getConstantsSize(C.enum_kernel(kernel))
}

// Block on stream's download and return any errors
// This also checks the CGBN error report (presumably this is where things should be checked, if not now, then in the future, to see whether they're in the group or not. However this may not(?) be doable if everything is in Montgomery space.)
func get(stream Stream) error {
	cErr := C.getResults(stream.s)
	err := goError(cErr)
	return err
}

// Reset the CUDA device
// Hopefully this will allow the CUDA profile to be gotten in the graphical profiler
//func resetDevice() error {
//	errString := C.resetDevice()
//	err := goError(errString)
//	return err
//}

// putBits() copies bits from one array to another and right-pads any remaining words with zeroes
func putBits(dst large.Bits, src large.Bits, n int) {
	copy(dst, src)
	for i := len(src); i < len(dst) && i < n; i++ {
		dst[i] = 0
	}
}

func initCuda() error {
	var err error
	cudaDone.Do(func() {
		errString := C.initCuda()
		err = goError(errString)
	})
	return err
}
