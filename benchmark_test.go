package main

import (
	"gitlab.com/elixxir/crypto/cyclic"
	"gitlab.com/elixxir/crypto/large"
	"math/rand"
	"testing"
)

func generateBenchmarkInputMem(g *cyclic.Group, xNumBytes, yNumBytes, n int, byteSizePerBN uint64) []byte {
	inputMem := make([]byte, 0, int(byteSizePerBN)*n*2)
	xBuf := make([]byte, xNumBytes)
	yBuf := make([]byte, yNumBytes)
	rng := rand.New(rand.NewSource(8074))
	for i := 0; i < n; i++ {
		_, err := rng.Read(xBuf)
		if err != nil {
			panic(err)
		}
		x := g.NewIntFromBytes(xBuf)
		inputMem = append(inputMem, x.CGBNMem(byteSizePerBN*8)...)
		_, err = rng.Read(yBuf)
		if err != nil {
			panic(err)
		}
		g.NewIntFromBytes(yBuf)
		y := g.NewIntFromBytes(yBuf)
		inputMem = append(inputMem, y.CGBNMem(byteSizePerBN*8)...)
	}
	return inputMem
}

func makeTestGroup2048() *cyclic.Group {
	return cyclic.NewGroup(
		large.NewIntFromString("FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3DC2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F83655D23DCA3AD961C62F356208552BB9ED529077096966D670C354E4ABC9804F1746C08CA18217C32905E462E36CE3BE39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9DE2BCBF6955817183995497CEA956AE515D2261898FA051015728E5A8AACAA68FFFFFFFFFFFFFFFF", 16),
		large.NewInt(2),
		large.NewInt(1),
	)
}

func makeTestGroup4096() *cyclic.Group {
	p := large.NewIntFromString("FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3DC2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F83655D23DCA3AD961C62F356208552BB9ED529077096966D670C354E4ABC9804F1746C08CA18217C32905E462E36CE3BE39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9DE2BCBF6955817183995497CEA956AE515D2261898FA051015728E5A8AAAC42DAD33170D04507A33A85521ABDF1CBA64ECFB850458DBEF0A8AEA71575D060C7DB3970F85A6E1E4C7ABF5AE8CDB0933D71E8C94E04A25619DCEE3D2261AD2EE6BF12FFA06D98A0864D87602733EC86A64521F2B18177B200CBBE117577A615D6C770988C0BAD946E208E24FA074E5AB3143DB5BFCE0FD108E4B82D120A92108011A723C12A787E6D788719A10BDBA5B2699C327186AF4E23C1A946834B6150BDA2583E9CA2AD44CE8DBBBC2DB04DE8EF92E8EFC141FBECAA6287C59474E6BC05D99B2964FA090C3A2233BA186515BE7ED1F612970CEE2D7AFB81BDD762170481CD0069127D5B05AA993B4EA988D8FDDC186FFB7DC90A6C08F4DF435C934063199FFFFFFFFFFFFFFFF", 16)
	return cyclic.NewGroup(
		p,
		large.NewInt(2),
		large.NewInt(1),
	)
}

// x**y, x and y are both 2048 bits long
/*func BenchmarkPowmCUDA2048_2048(b *testing.B) {
	var results []byte
	const bitLen = 2048
	const byteLen = bitLen / 8
	g := makeTestGroup2048()
	inputMem := generateBenchmarkInputMem(g, byteLen, byteLen, b.N, byteLen)
	pMem := g.GetP().CGBNMem(bitLen)

	// Maybe don't load the library twice?
	err := loadLibrary()
	b.ResetTimer()
	// We'll run the exponentiation for the whole array in one chunk
	// It might be possible to run another benchmark that does two or more
	// chunks instead, which could be faster if the call could be made
	// asynchronous (which should be possible)
	results, err = powm_2048(pMem, inputMem, uint32(b.N))
	if err != nil {
		b.Fatal(err)
	}
	b.StopTimer()
	// This benchmark doesn't include converting resulting memory back to cyclic ints
	b.Log(results[0])
}

// x**y, x is 2048 bits long, y is 256 bits long
func BenchmarkPowmCUDA2048_256(b *testing.B) {
	var results []byte
	const xBitLen = 2048
	const xByteLen = xBitLen / 8
	const yBitLen = 256
	const yByteLen = yBitLen / 8
	g := makeTestGroup2048()
	inputMem := generateBenchmarkInputMem(g, xByteLen, yByteLen, b.N, xByteLen)
	pMem := g.GetP().CGBNMem(bitLen)

	// Maybe don't load the library twice?
	err := loadLibrary()
	b.ResetTimer()
	// We'll run the exponentiation for the whole array in one chunk
	// It might be possible to run another benchmark that does two or more
	// chunks instead, which could be faster if the call could be made
	// asynchronous (which should be possible)
	results, err = powm_2048(pMem, inputMem, uint32(b.N))
	if err != nil {
		b.Fatal(err)
	}
	b.StopTimer()
	// This benchmark doesn't include converting resulting memory back to cyclic ints
	b.Log(results[0])
} */

func BenchmarkPowmCUDA4096_4096(b *testing.B) {
	var results []byte
	const bitLen = 4096
	const byteLen = bitLen / 8
	g := makeTestGroup4096()
	inputMem := generateBenchmarkInputMem(g, byteLen, byteLen, b.N, byteLen)
	pMem := g.GetP().CGBNMem(bitLen)

	// Maybe don't load the library twice?
	err := startProfiling()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	// We'll run the exponentiation for the whole array in one chunk
	// It might be possible to run another benchmark that does two or more
	// chunks instead, which could be faster if the call could be made
	// asynchronous (which should be possible)
	results, err = powm_4096(pMem, inputMem, uint32(b.N))
	if err != nil {
		b.Fatal(err)
	}
	b.StopTimer()
	// This benchmark doesn't include converting resulting memory back to cyclic ints
	b.Log(results[0])
	// Write out any cached profiling data
	err = stopProfiling()
	// Maybe we need to start profiling again for the next run?
	if err != nil {
		b.Fatal(err)
	}
	err = resetDevice();
	if err != nil {
		b.Fatal(err)
	}
}

// x**y, x is 2048 bits long, y is 256 bits long
func BenchmarkPowmCUDA4096_256(b *testing.B) {
	var results []byte
	const xBitLen = 4096
	const xByteLen = xBitLen / 8
	const yBitLen = 256
	const yByteLen = yBitLen / 8
	g := makeTestGroup4096()
	inputMem := generateBenchmarkInputMem(g, xByteLen, yByteLen, b.N, xByteLen)
	pMem := g.GetP().CGBNMem(bitLen)

	// Maybe don't load the library twice?
	err := startProfiling()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	// We'll run the exponentiation for the whole array in one chunk
	// It might be possible to run another benchmark that does two or more
	// chunks instead, which could be faster if the call could be made
	// asynchronous (which should be possible)
	results, err = powm_4096(pMem, inputMem, uint32(b.N))
	if err != nil {
		b.Fatal(err)
	}
	b.StopTimer()
	// This benchmark doesn't include converting resulting memory back to cyclic ints
	b.Log(results[0])
	// Write out any cached profiling data
	err = stopProfiling();
	if err != nil {
		b.Fatal(err)
	}
	err = resetDevice();
	if err != nil {
		b.Fatal(err)
	}
}
