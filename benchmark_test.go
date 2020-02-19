package gpumaths

import (
	"gitlab.com/elixxir/crypto/cyclic"
	"gitlab.com/elixxir/crypto/large"
	"math/rand"
	"testing"
)

func makeTestGroup4096() *cyclic.Group {
	p := large.NewIntFromString("FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3DC2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F83655D23DCA3AD961C62F356208552BB9ED529077096966D670C354E4ABC9804F1746C08CA18217C32905E462E36CE3BE39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9DE2BCBF6955817183995497CEA956AE515D2261898FA051015728E5A8AAAC42DAD33170D04507A33A85521ABDF1CBA64ECFB850458DBEF0A8AEA71575D060C7DB3970F85A6E1E4C7ABF5AE8CDB0933D71E8C94E04A25619DCEE3D2261AD2EE6BF12FFA06D98A0864D87602733EC86A64521F2B18177B200CBBE117577A615D6C770988C0BAD946E208E24FA074E5AB3143DB5BFCE0FD108E4B82D120A92108011A723C12A787E6D788719A10BDBA5B2699C327186AF4E23C1A946834B6150BDA2583E9CA2AD44CE8DBBBC2DB04DE8EF92E8EFC141FBECAA6287C59474E6BC05D99B2964FA090C3A2233BA186515BE7ED1F612970CEE2D7AFB81BDD762170481CD0069127D5B05AA993B4EA988D8FDDC186FFB7DC90A6C08F4DF435C934063199FFFFFFFFFFFFFFFF", 16)
	return cyclic.NewGroup(
		p,
		large.NewInt(2),
	)
}

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
