package main

import (
	"gitlab.com/elixxir/crypto/cyclic"
	"gitlab.com/elixxir/crypto/large"
	"math/rand"
	"testing"
)

// This function creates a channel that returns chunks of memory
// that are valid inputs for the exponentiation kernel.
// Because the speed can depend on the number of non-zero bits,
// the caller specifies the length and capacity of the numbers.
func benchmarkInputMemGenerator(g *cyclic.Group, n int, byteSizePerBN uint64, numBytes ...int) chan []byte {
	// New input mem is generated and put out on this channel
	result := make(chan []byte)
	go func() {
		// Completely arbitrary seed to get a consistent set of input data for test running
		seed := int64(8074)
		rng := rand.New(rand.NewSource(seed))
		for {
			inputMem := make([]byte, 0, int(byteSizePerBN)*n*2)
			for i := 0; i < n; i++ {
				for j := 0; j < len(numBytes); j++ {
					buf := make([]byte, numBytes[j])
					_, err := rng.Read(buf)
					if err != nil {
						panic(err)
					}
					input := g.NewIntFromBytes(buf)
					inputMem = append(inputMem, input.CGBNMem(byteSizePerBN*8)...)
				}
			}
			result <- inputMem
		}
	}()
	return result
}

func makeTestGroup4096() *cyclic.Group {
	p := large.NewIntFromString("FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3DC2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F83655D23DCA3AD961C62F356208552BB9ED529077096966D670C354E4ABC9804F1746C08CA18217C32905E462E36CE3BE39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9DE2BCBF6955817183995497CEA956AE515D2261898FA051015728E5A8AAAC42DAD33170D04507A33A85521ABDF1CBA64ECFB850458DBEF0A8AEA71575D060C7DB3970F85A6E1E4C7ABF5AE8CDB0933D71E8C94E04A25619DCEE3D2261AD2EE6BF12FFA06D98A0864D87602733EC86A64521F2B18177B200CBBE117577A615D6C770988C0BAD946E208E24FA074E5AB3143DB5BFCE0FD108E4B82D120A92108011A723C12A787E6D788719A10BDBA5B2699C327186AF4E23C1A946834B6150BDA2583E9CA2AD44CE8DBBBC2DB04DE8EF92E8EFC141FBECAA6287C59474E6BC05D99B2964FA090C3A2233BA186515BE7ED1F612970CEE2D7AFB81BDD762170481CD0069127D5B05AA993B4EA988D8FDDC186FFB7DC90A6C08F4DF435C934063199FFFFFFFFFFFFFFFF", 16)
	return cyclic.NewGroup(
		p,
		large.NewInt(2),
	)
}

func BenchmarkPowmCUDA4096_4096(b *testing.B) {
	var results []byte
	const bitLen = 4096
	const byteLen = bitLen / 8
	g := makeTestGroup4096()
	inputMem := benchmarkInputMemGenerator(g, b.N, byteLen, byteLen, byteLen)
	pMem := g.GetP().CGBNMem(bitLen)

	err := startProfiling()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	// We'll run the exponentiation for the whole array in one chunk
	// It might be possible to run another benchmark that does two or more
	// chunks instead, which could be faster if the call could be made
	// asynchronous (which should be possible)
	results, err = powm4096(pMem, <-inputMem, b.N)
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
	err = resetDevice()
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
	inputMem := benchmarkInputMemGenerator(g,  b.N, xByteLen, xByteLen, yByteLen)
	pMem := g.GetP().CGBNMem(bnSizeBits)

	err := startProfiling()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	// We'll run the exponentiation for the whole array in one chunk
	// It might be possible to run another benchmark that does two or more
	// chunks instead, which could be faster if the call could be made
	// asynchronous (which should be possible)
	results, err = powm4096(pMem, <-inputMem, b.N)
	if err != nil {
		b.Fatal(err)
	}
	b.StopTimer()
	// This benchmark doesn't include converting resulting memory back to cyclic ints
	b.Log(results[0])
	// Write out any cached profiling data
	err = stopProfiling()
	if err != nil {
		b.Fatal(err)
	}
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
	pMem := g.GetP().CGBNMem(bnSizeBits)

	// This benchmark is "cheating" compared to the last one by doing allocations before the timer's reset
	// Use two streams with 2048 items per kernel launch
	numItems := 32768
	inputMem := benchmarkInputMemGenerator(g,  numItems, xByteLen, xByteLen, yByteLen)

	numStreams := 2
	streams, err := createStreams(numStreams, numItems, kernelPowmOdd)
	workingStream := streams[0]
	waitingStream := streams[1]
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	remainingItems := b.N
	for i := 0; i < b.N; i += numItems {
		// If part of a chunk remains, only upload that part
		remainingItems = b.N - i
		numItemsToUpload := numItems
		if remainingItems < numItems {
			numItemsToUpload = remainingItems
		}
		err = upload(pMem, <-inputMem, numItemsToUpload, workingStream, kernelPowmOdd)
		if err != nil {
			b.Fatal(err)
		}
		err = run(workingStream, kernelPowmOdd)
		if err != nil {
			b.Fatal(err)
		}
		// Download items from the other stream after starting work in this stream
		err := download(waitingStream)
		if err != nil {
			b.Fatal(err)
		}
		// Copy inputs from the stream before that (this is required for meaningful usage)
		// The number of items isn't always correct, but it shouldn't make a big difference to the benchmark.
		_, err = getResults(waitingStream, numItems, kernelPowmOdd)
		if err != nil {
			b.Fatal(err)
		}
		// Switch streams
		workingStream, waitingStream = waitingStream, workingStream
	}
	// Download the last results
	err = download(waitingStream)
	if err != nil {
		b.Fatal(err)
	}
	_, err = getResults(waitingStream, numItems, kernelPowmOdd)
	if err != nil {
		b.Fatal(err)
	}
	b.StopTimer()
	err = destroyStreams(streams)
	if err != nil {
		b.Fatal(err)
	}

	err = stopProfiling()
	if err != nil {
		b.Fatal(err)
	}
	err = resetDevice()
	if err != nil {
		b.Fatal(err)
	}
}

func BenchmarkElgamalCUDA4096_256_streams(b *testing.B) {
	const longBitLen = 4096
	const longByteLen = longBitLen / 8
	const shortBitLen = 256
	const shortByteLen = shortBitLen / 8
	g := makeTestGroup4096()
	// Start with a pseudorandom cypher public key
	cpkBytes := make([]byte, longByteLen)
	rand.Read(cpkBytes)
	cpk := g.NewIntFromBytes(cpkBytes)
	pMem := stageElgamalConstants(g, cpk)

	// This benchmark is "cheating" compared to the last one by doing allocations before the timer's reset
	// Use two streams with 2048 items per kernel launch
	numItems := 32768
	// the private key is the only input that can be assumed to be short
	inputMem := benchmarkInputMemGenerator(g, numItems, longByteLen, shortByteLen, longByteLen, longByteLen, longByteLen)

	numStreams := 2
	streams, err := createStreams(numStreams, numItems, kernelElgamal)
	workingStream := streams[0]
	waitingStream := streams[1]
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	remainingItems := b.N
	for i := 0; i < b.N; i += numItems {
		// If part of a chunk remains, only upload that part
		remainingItems = b.N - i
		numItemsToUpload := numItems
		if remainingItems < numItems {
			numItemsToUpload = remainingItems
		}
		err = upload(pMem, <-inputMem, numItemsToUpload, workingStream, kernelElgamal)
		if err != nil {
			b.Fatal(err)
		}
		err = run(workingStream, kernelElgamal)
		if err != nil {
			b.Fatal(err)
		}
		// Download items from the other stream after starting work in this stream
		err := download(waitingStream)
		if err != nil {
			b.Fatal(err)
		}
		// Copy inputs from the stream before that (this is required for meaningful usage)
		// The number of items isn't always correct, but it shouldn't make a big difference to the benchmark.
		_, err = getResults(waitingStream, numItems, kernelElgamal)
		if err != nil {
			b.Fatal(err)
		}
		// Switch streams
		workingStream, waitingStream = waitingStream, workingStream
	}
	// Download the last results
	err = download(waitingStream)
	if err != nil {
		b.Fatal(err)
	}
	_, err = getResults(waitingStream, numItems, kernelElgamal)
	if err != nil {
		b.Fatal(err)
	}
	b.StopTimer()
	err = destroyStreams(streams)
	if err != nil {
		b.Fatal(err)
	}

	err = stopProfiling()
	if err != nil {
		b.Fatal(err)
	}
	err = resetDevice()
	if err != nil {
		b.Fatal(err)
	}
}
