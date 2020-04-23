//+build linux,cuda

package gpumaths

import "unsafe"

// TODO Functions that currently take a stream as unsafe.Pointer should instead have a stream as the receiver
type Stream struct {
	// Pointer to stream and associated data, usable only on the C side
	s               unsafe.Pointer
	maxSlotsElGamal int
	maxSlotsExp     int
	maxSlotsReveal  int
	maxSlotsStrip   int
}

func (s *Stream) GetMaxSlotsElGamal() int {
	return s.maxSlotsElGamal
}

func (s *Stream) GetMaxSlotsExp() int {
	return s.maxSlotsExp
}

func (s *Stream) GetMaxSlotsReveal() int {
	return s.maxSlotsReveal
}

func (s *Stream) GetMaxSlotsStrip() int {
	return s.maxSlotsStrip
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

func MaxSlots(memSize int, op int) int {
	var constantsSize, slotSize int
	switch op {
	case kernelPowmOdd:
		constantsSize = getConstantsSizePowm4096()
		slotSize = getInputsSizePowm4096() + getOutputsSizePowm4096()
	case kernelElgamal:
		constantsSize = getConstantsSizeElgamal()
		slotSize = getInputsSizeElgamal() + getOutputsSizeElgamal()
	case kernelReveal:
		constantsSize = getConstantsSizeReveal()
		slotSize = getInputsSizeReveal() + getOutputsSizeReveal()
	case kernelStrip:
		constantsSize = getConstantsSizeStrip()
		slotSize = getInputsSizeStrip() + getOutputsSizeStrip()
	}
	memForSlots := memSize - constantsSize
	if memForSlots < 0 {
		return 0
	} else {
		return memForSlots / slotSize
	}
}

func streamSizeContaining(numItems int, kernel int) int {
	switch kernel {
	case kernelPowmOdd:
		return getInputsSizePowm4096()*numItems + getOutputsSizePowm4096()*numItems + getConstantsSizePowm4096()
	case kernelElgamal:
		return getInputsSizeElgamal()*numItems + getOutputsSizeElgamal()*numItems + getConstantsSizeElgamal()
	case kernelReveal:
		return getInputsSizeReveal()*numItems + getOutputsSizeReveal()*numItems + getConstantsSizeReveal()
	case kernelStrip:
		return getInputsSizeStrip()*numItems + getOutputsSizeStrip()*numItems + getConstantsSizeStrip()
	}
	return 0
}
