//+build linux,cuda

package gpumaths

import (
	"math/rand"
	"testing"
)

func BenchmarkPowmCUDA4096_4096(b *testing.B) {
	g := makeTestGroup4096()
	numSlots := b.N
	input := ExpInput{
		Slots:   make([]ExpInputSlot, numSlots),
		Modulus: g.GetPBytes(),
	}
	for i := 0; i < numSlots; i++ {
		input.Slots[i] = ExpInputSlot{
			Base:     g.Random(g.NewInt(1)).Bytes(),
			Exponent: g.Random(g.NewInt(1)).Bytes(),
		}
	}

	stream, err := createStreams(1, streamSizeContaining(numSlots, kernelPowmOdd))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	// We'll run the exponentiation for the whole array in one chunk
	// It might be possible to run another benchmark that does two or more
	// chunks instead, which could be faster if the call could be made
	// asynchronous (which should be possible)
	resultChan := Exp(input, stream[0])
	result := <-resultChan
	if result.Err != nil {
		b.Fatal(result.Err)
	}
	b.StopTimer()
	// Write out any cached profiling data
	err = resetDevice()
	if err != nil {
		b.Fatal(err)
	}
}

// x**y, x is 2048 bits long, y is 256 bits long
func BenchmarkPowmCUDA4096_256(b *testing.B) {
	const xBitLen = 4096
	const xByteLen = xBitLen / 8
	const yBitLen = 256
	const yByteLen = yBitLen / 8
	g := makeTestGroup4096()

	numSlots := b.N
	streams, err := createStreams(1, streamSizeContaining(numSlots, kernelPowmOdd))
	input := ExpInput{
		Slots:   make([]ExpInputSlot, numSlots),
		Modulus: g.GetPBytes(),
	}

	for i := 0; i < numSlots; i++ {
		input.Slots[i].Exponent = g.Random(g.NewInt(1)).Bytes()[480:]
		input.Slots[i].Base = g.Random(g.NewInt(1)).Bytes()
	}
	b.ResetTimer()
	// We'll run the exponentiation for the whole array in one chunk
	// It might be possible to run another benchmark that does two or more
	// chunks instead, which could be faster if the call could be made
	// asynchronous (which should be possible)
	resultChan := Exp(input, streams[0])
	result := <-resultChan
	if result.Err != nil {
		b.Fatal(result.Err)
	}
	b.StopTimer()
	// This benchmark doesn't include converting resulting memory back to cyclic ints
	// Write out any cached profiling data
	err = resetDevice()
	if err != nil {
		b.Fatal(err)
	}
}

func BenchmarkPowmCUDA4096_256_streams(b *testing.B) {
	const xBitLen = 4096
	const xByteLen = xBitLen / 8
	const yBitLen = 256
	const yByteLen = yBitLen / 8
	g := makeTestGroup4096()
	// Use two streams with 32k items per kernel launch
	numItems := 32768

	streamPool, err := NewStreamPool(2, streamSizeContaining(numItems, kernelPowmOdd))
	if err != nil {
		b.Fatal(err)
	}
	// Using prng because the cryptographically secure RNG used by the group is too slow to feed the GPU
	rng := rand.New(rand.NewSource(5))
	b.ResetTimer()
	remainingItems := b.N
	for i := 0; i < b.N; i += numItems {
		// If part of a chunk remains, only upload that part
		remainingItems = b.N - i
		numItemsToUpload := numItems
		if remainingItems < numItems {
			numItemsToUpload = remainingItems
		}
		input := ExpInput{
			Slots:   make([]ExpInputSlot, numItemsToUpload),
			Modulus: g.GetPBytes(),
		}
		// Hopefully random number generation doesn't bottleneck things!
		for j := 0; j < numItemsToUpload; j++ {
			// Unfortunately, we can't just generate things using the group, because it's too slow
			base := make([]byte, xByteLen)
			exponent := make([]byte, yByteLen)
			rng.Read(base)
			rng.Read(exponent)
			for !g.BytesInside(base, exponent) {
				rng.Read(base)
				rng.Read(exponent)
			}
			input.Slots[j] = ExpInputSlot{
				Base:     base,
				Exponent: exponent,
			}
		}
		stream := streamPool.TakeStream()
		resultChan := Exp(input, stream)
		go func() {
			result := <-resultChan
			streamPool.ReturnStream(stream)
			if result.Err != nil {
				b.Fatal(result.Err)
			}
		}()
	}
	// Empty the pool to make sure results have all been downloaded
	streamPool.TakeStream()
	streamPool.TakeStream()
	b.StopTimer()
	err = streamPool.Destroy()
	if err != nil {
		b.Fatal(err)
	}

	err = resetDevice()
	if err != nil {
		b.Fatal(err)
	}
}

func BenchmarkElGamalCUDA4096_256_streams(b *testing.B) {
	const xBitLen = 4096
	const xByteLen = xBitLen / 8
	const yBitLen = 256
	const yByteLen = yBitLen / 8
	g := makeTestGroup4096()
	// Use two streams with 32k items per kernel launch
	numItems := 32768

	// OK, this shouldn't cause the test to run forever if the stream size is smaller than it should be (like this)
	// In real-world usage, the number of slots passed in should be determined by what the stream supports
	//  (i.e. check stream.MaxSlotsElgamal)
	streamPool, err := NewStreamPool(2, streamSizeContaining(numItems, kernelElgamal))
	if err != nil {
		b.Fatal(err)
	}
	// Using prng because the cryptographically secure RNG used by the group is too slow to feed the GPU
	rng := rand.New(rand.NewSource(5))
	b.ResetTimer()
	remainingItems := b.N
	for i := 0; i < b.N; i += numItems {
		// If part of a chunk remains, only upload that part
		remainingItems = b.N - i
		numItemsToUpload := numItems
		if remainingItems < numItems {
			numItemsToUpload = remainingItems
		}
		input := ElGamalInput{
			Slots:           make([]ElGamalInputSlot, numItemsToUpload),
			PublicCypherKey: g.Random(g.NewInt(1)).Bytes(),
			Prime:           g.GetPBytes(),
			G:               g.GetG().Bytes(),
		}
		// Hopefully random number generation doesn't bottleneck things!
		for j := 0; j < numItemsToUpload; j++ {
			// Unfortunately, we can't just generate things using the group, because it's too slow
			key := make([]byte, xByteLen)
			ecrKey := make([]byte, xByteLen)
			cypher := make([]byte, xByteLen)
			privateKey := make([]byte, yByteLen)
			rng.Read(key)
			rng.Read(ecrKey)
			rng.Read(cypher)
			rng.Read(privateKey)
			for !g.BytesInside(key, ecrKey, cypher, privateKey) {
				rng.Read(key)
				rng.Read(ecrKey)
				rng.Read(cypher)
				rng.Read(privateKey)
			}
			input.Slots[j] = ElGamalInputSlot{
				PrivateKey: privateKey,
				Key:        key,
				EcrKey:     ecrKey,
				Cypher:     cypher,
			}
		}
		stream := streamPool.TakeStream()
		resultChan := ElGamal(input, stream)
		go func() {
			result := <-resultChan
			streamPool.ReturnStream(stream)
			if result.Err != nil {
				b.Fatal(result.Err)
			}
		}()
	}
	// Empty the pool to make sure results have all been downloaded
	streamPool.TakeStream()
	streamPool.TakeStream()
	b.StopTimer()
	err = streamPool.Destroy()
	if err != nil {
		b.Fatal(err)
	}

	err = resetDevice()
	if err != nil {
		b.Fatal(err)
	}
}
