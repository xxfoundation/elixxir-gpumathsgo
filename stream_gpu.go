///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

//+build linux,gpu

package gpumaths

/*
#cgo CFLAGS: -I./cgbnBindings/powm -I/opt/xxnetwork/include
#cgo LDFLAGS: -L/opt/xxnetwork/lib -lpowmosm75 -Wl,-rpath,./lib:/opt/xxnetwork/lib
#include <powm_odd_export.h>
#include <stdlib.h>
#include <string.h>
*/
import "C"
import (
	"git.xx.network/xx_network/crypto/large"
	"unsafe"
)

// TODO Functions that currently take a stream as unsafe.Pointer should instead have a stream as the receiver
type Stream struct {
	// Pointer to stream and associated data, usable only on the C side
	s unsafe.Pointer
	// This byte slice contains the entire range of the CPU buffer that this stream can use
	cpuData []byte
	// Same data but in words!
	cpuDataWords large.Bits
}

// Return the portion of the stream's CPU memory that's used for outputs
// Outputs come after inputs and constants
func (s *Stream) getCpuOutputsWords(g gpumathsEnv, kernel C.enum_kernel, numItems int) large.Bits {
	start := g.getConstantsSizeWords(kernel) + g.getInputSizeWords(kernel)*numItems
	end := start + g.getOutputSizeWords(kernel)*numItems
	return s.cpuDataWords[start:end]
}

// Inputs come after constants and before outputs
func (s *Stream) getCpuInputsWords(g gpumathsEnv, kernel C.enum_kernel, numItems int) large.Bits {
	start := g.getConstantsSizeWords(kernel)
	end := start + g.getInputSizeWords(kernel)*numItems
	return s.cpuDataWords[start:end]
}

// Constants exist at the very start of the buffer
func (s *Stream) getCpuConstantsWords(g gpumathsEnv, kernel C.enum_kernel) large.Bits {
	return s.cpuDataWords[:g.getConstantsSizeWords(kernel)]
}

// Optional improvements:
//  - create streams with high priority to speed up kernels used for realtime
type StreamPool struct {
	// Used to prevent concurrent access to streams
	streamChan chan Stream
	// Used to time-bound stream deletion. These are the same streams that you can get from the channel
	streams []Stream
}

// numStreams: Number of streams per device. 2 is usually fine
func NewStreamPool(numStreams int, memSize int) (*StreamPool, error) {
	// We should be able to init CUDA here and have it work, right?
	err := initCuda()
	if err != nil {
		return nil, err
	}
	// Each stream should support all operations if there's enough memory available
	var result StreamPool
	streams, err := createStreams(numStreams, memSize)
	if err != nil {
		// TODO Destroy streams first before returning
		return nil, err
	}
	result.streams = streams
	result.streamChan = make(chan Stream, len(streams))
	for i := range result.streams {
		result.streamChan <- result.streams[i]
	}

	return &result, err
}

// If you need to, it's also possible to create an equivalent method that times out
// This method gets a stream from the channel
func (sm *StreamPool) TakeStream() Stream {
	return <-sm.streamChan
}

func (sm *StreamPool) ReturnStream(s Stream) {
	if s.s != nil {
		sm.streamChan <- s
	}
}

// Destroy all the stream pool's streams
// This doesn't wait on any work to finish before destroying the streams.
// If it's a problem in the future I'll have this method empty the channel before destroying the streams.
func (sm *StreamPool) Destroy() error {
	return destroyStreams(sm.streams)
}
