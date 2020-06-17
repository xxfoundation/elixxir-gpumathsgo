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
	"errors"
	"reflect"
	"unsafe"
)

// Package C enum in golang for testing, possible export?
const (
	bnSizeBits    = 4096
	bnSizeBytes   = bnSizeBits / 8
	bnLength      = 4096
	bnLengthBytes = bnLength / 8
)

// Create byte slice viewing memory at a certain memory address with a
// certain length
// Here be dragons
func toSlice(pointer unsafe.Pointer, size int) []byte {
	return *(*[]byte)(unsafe.Pointer(
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
		createStreamResult := C.createStream(streamCreateInfo)
		stream := Stream{
			s: createStreamResult.result,
		}
		if stream.s != nil {
			streams = append(streams, stream)
		}
		if createStreamResult.error != nil {
			// Try to destroy all created streams to avoid leaking memory
			for j := 0; j < len(streams); j++ {
				C.destroyStream(streams[j].s)
			}
			return nil, goError(createStreamResult.error)
		}
	}

	maxSlotsElGamal := MaxSlots(capacity, kernelElgamal)
	maxSlotsExp := MaxSlots(capacity, kernelPowmOdd)
	maxSlotsReveal := MaxSlots(capacity, kernelReveal)
	maxSlotsStrip := MaxSlots(capacity, kernelStrip)
	maxSlotsMul2 := MaxSlots(capacity, kernelMul2)
	for i := 0; i < numStreams; i++ {
		streams[i].maxSlotsElGamal = maxSlotsElGamal
		streams[i].maxSlotsExp = maxSlotsExp
		streams[i].maxSlotsReveal = maxSlotsReveal
		streams[i].maxSlotsStrip = maxSlotsStrip
		streams[i].maxSlotsMul2 = maxSlotsMul2
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
func put(stream Stream, whichToRun C.enum_kernel, numSlots int) error {
	uploadError := C.upload(C.uint(numSlots), stream.s, whichToRun)
	if uploadError != nil {
		return goError(uploadError)
	} else {
		return nil
	}
}

// Can you use the C type like this?
// Might need to redefine enumeration in Golang
func run(stream Stream) error {
	return goError(C.run(stream.s))
}

// Enqueue a download for this stream after execution finishes
// Doesn't actually block for the download
func download(stream Stream) error {
	return goError(C.download(stream.s))
}

// Wait for this stream's download to finish and return a pointer to the results
// This also checks the CGBN error report (presumably this is where things should be checked, if not now, then in the future, to see whether they're in the group or not. However this may not(?) be doable if everything is in Montgomery space.)
// TODO Copying results to Golang should no longer be the responsibility of this method
//  Instead, this can be done in the exported integration method, and it can be copied from
//  the results buffer directly. The length of the results buffer could be another
//  field of the struct as well, although it would be better to allocate that earlier.
func get(stream Stream) error {
	return goError(C.getResults(stream.s))
}

// Reset the CUDA device
// Hopefully this will allow the CUDA profile to be gotten in the graphical profiler
func resetDevice() error {
	errString := C.resetDevice()
	err := goError(errString)
	return err
}

// TODO better to use an offset or slice the header in different places?
// Puts an integer (in bytes) into a buffer
// Check bounds here? Any safety available?
// Src and dst should be different memory areas. This isn't meant to work meaningfully if the buffers overlap.
// n is the length in bytes of the int in the destination area
// if src is too short, an area of dst will be overwritten with zeroes for safety reasons (right-padded)
func putInt(dst []byte, src []byte, n int) {
	n2 := len(src)
	for i := 0; i < n2; i++ {
		dst[n2-i-1] = src[i]
	}
	for i := n2; i < n; i++ {
		dst[i] = 0
	}
}
