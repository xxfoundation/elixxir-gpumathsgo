package gpumaths

import "gitlab.com/elixxir/crypto/cyclic"

type ElGamalInputSlot struct {
	PrivateKey []byte
	Key        []byte
	EcrKey     []byte
	Cypher     []byte
}

type ElGamalInput struct {
	Slots           []ElGamalInputSlot
	PublicCypherKey []byte
	Prime           []byte
	G               []byte
}

type ElGamalResultSlot struct {
	EcrKey []byte
	Cypher []byte
}

type ElGamalResult struct {
	Slots []ElGamalResultSlot
	Err   error
}

type ExpInputSlot struct {
	Base     []byte
	Exponent []byte
}

type ExpInput struct {
	Slots []ExpInputSlot
	Modulus []byte
}

type ExpResult struct {
	Results [][]byte
	Err   error
}

// Implement cryptop interface for ExpChunk
type ExpChunkPrototype func(p *StreamPool, g *cyclic.Group, x, y, z *cyclic.IntBuffer) (*cyclic.IntBuffer, error)

func (ExpChunkPrototype) GetName() string {
	return "ExpChunk"
}

// 1 for now, experiment later - will partial chunks still be ok if this is higher? is it possible to drive this from the stream size? or does this do something i'm not expecting and can't reason about?
func (ExpChunkPrototype) GetInputSize() uint32 {
	return 256
}

// Type necessary to implement cryptop interface
type ElGamalChunkPrototype func(p *StreamPool, g *cyclic.Group, key, privateKey *cyclic.IntBuffer, publicCypherKey *cyclic.Int, ecrKey, cypher *cyclic.IntBuffer) error

func (ElGamalChunkPrototype) GetInputSize() uint32 {
	return 128
}

func (ElGamalChunkPrototype) GetName() string {
	return "ElGamalChunk"
}
