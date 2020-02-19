package gpumaths

import (
	"gitlab.com/elixxir/crypto/cyclic"
)

// Implement cryptop interface for ExpChunk
type ExpChunkPrototype func(p *StreamPool, g *cyclic.Group, x, y, z *cyclic.IntBuffer) (*cyclic.IntBuffer, error)

// Using this function doesn't allow you to do other things while waiting on the kernel to finish
// Perform exponentiation for two operands and place the result in z (which is also returned)
var ExpChunk ExpChunkPrototype = func(p *StreamPool, g *cyclic.Group, x, y, z *cyclic.IntBuffer) (*cyclic.IntBuffer, error) {
	// Populate exp inputs
	numSlots := z.Len()
	input := ExpInput{
		Slots:   make([]ExpInputSlot, numSlots),
		Modulus: g.GetPBytes(),
	}
	for i := uint32(0); i < uint32(numSlots); i++ {
		input.Slots[i] = ExpInputSlot{
			Base:     x.Get(i).Bytes(),
			Exponent: y.Get(i).Bytes(),
		}
	}

	// Run kernel on the inputs, simply using smaller chunks if passed chunk size exceeds buffer space in stream
	stream := p.TakeStream()
	defer p.ReturnStream(stream)
	for i := 0; i < numSlots; i += stream.MaxSlotsExp {
		sliceEnd := i
		// Don't slice beyond the end of the input slice
		if i+stream.MaxSlotsExp <= numSlots {
			sliceEnd += stream.MaxSlotsExp
		} else {
			sliceEnd = numSlots
		}
		thisInput := ExpInput{
			Slots:   input.Slots[i:sliceEnd],
			Modulus: input.Modulus,
		}
		result := <-Exp(thisInput, stream)
		if result.Err != nil {
			return z, result.Err
		}
		// Populate with results
		for j := range result.Results {
			g.SetBytes(z.Get(uint32(i+j)), result.Results[j])
		}
	}

	// If there were no errors, we return z
	return z, nil
}

func (ExpChunkPrototype) GetName() string {
	return "ExpChunk"
}

// 1 for now, experiment later - will partial chunks still be ok if this is higher? is it possible to drive this from the stream size? or does this do something i'm not expecting and can't reason about?
func (ExpChunkPrototype) GetInputSize() uint32 {
	return 256
}

// Type necessary to implement cryptop interface
type ElGamalChunkPrototype func(p *StreamPool, g *cyclic.Group, key, privateKey *cyclic.IntBuffer, publicCypherKey *cyclic.Int, ecrKey, cypher *cyclic.IntBuffer) error

// Precondition: All int buffers must have the same length
// Perform the ElGamal operation on two int buffers
var ElGamalChunk ElGamalChunkPrototype = func(p *StreamPool, g *cyclic.Group, key, privateKey *cyclic.IntBuffer, publicCypherKey *cyclic.Int, ecrKey, cypher *cyclic.IntBuffer) error {
	// Populate ElGamal inputs
	numSlots := ecrKey.Len()
	input := ElGamalInput{
		Slots:           make([]ElGamalInputSlot, numSlots),
		PublicCypherKey: publicCypherKey.Bytes(),
		Prime:           g.GetPBytes(),
		G:               g.GetG().Bytes(),
	}
	for i := uint32(0); i < uint32(numSlots); i++ {
		input.Slots[i] = ElGamalInputSlot{
			PrivateKey: privateKey.Get(i).Bytes(),
			Key:        key.Get(i).Bytes(),
			EcrKey:     ecrKey.Get(i).Bytes(),
			Cypher:     cypher.Get(i).Bytes(),
		}
	}

	// Run kernel on the inputs
	stream := p.TakeStream()
	defer p.ReturnStream(stream)
	for i := 0; i < numSlots; i += stream.MaxSlotsElGamal {
		sliceEnd := i
		// Don't slice beyond the end of the input slice
		if i+stream.MaxSlotsElGamal <= numSlots {
			sliceEnd += stream.MaxSlotsElGamal
		} else {
			sliceEnd = numSlots
		}
		thisInput := ElGamalInput{
			Slots:           input.Slots[i:sliceEnd],
			Prime:           input.Prime,
			G:               input.G,
			PublicCypherKey: input.PublicCypherKey,
		}
		result := <-ElGamal(thisInput, stream)
		if result.Err != nil {
			return result.Err
		}
		// Populate with results
		for j := range result.Slots {
			g.SetBytes(ecrKey.Get(uint32(i+j)), result.Slots[j].EcrKey)
			g.SetBytes(cypher.Get(uint32(i+j)), result.Slots[j].Cypher)
		}
	}

	return nil
}

func (ElGamalChunkPrototype) GetInputSize() uint32 {
	return 128
}

func (ElGamalChunkPrototype) GetName() string {
	return "ElGamalChunk"
}
