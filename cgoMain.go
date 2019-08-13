package main

/*
#cgo LDFLAGS: -ldl
#include <stdio.h>
#include <stdint.h>
#include <dlfcn.h>
#include <stddef.h>
#include <stdlib.h>

// The powm implementation should return a struct that's sort of equivalent to this
struct powm_return_t {
	// Because of type incompatibility, CGO would return these values as an unsafe pointer anyway.
	// So there's no reason to try to get a type that's compatible with the C++ memory here.
	// It would just get converted to an unsafe pointer anyway.
	void *outputs;
	char *error;
};

// The following function pointers and handle get populated whenever loadLibrary() is called,
// and depopulated whenever unloadLibrary() is called.
void *dlHandle;

// Perform modular exponentiation on the passed instances.
// Returned instances will be populated with the result of the modular exponentiations.
// The returned struct includes an error string
// x ** y mod prime
// x and y are contiguous chunks of memory, where each 2048 bits contains a big number.
// TODO perf comparison for SoA/AoS
// Also, perf comparison for prime getting loaded from constant memory
struct powm_return_t*(*powmImpl_2048)(const void *prime, const void *instances, const uint32_t numInstances);

// Returns error string
// TODO: Should also set errno for cgo? (file not found type of thing)
//  Or, do the called methods do that already?
// Get errno for the file i/o error if the shared library can't be accessed or found
char* loadLibrary() {
	dlHandle = dlopen("./lib/libpowmosm75.so", RTLD_LAZY);
	char *error;
	// clear dlerror
	dlerror();
	if ((error = dlerror()) != NULL) {
		return error;
	}

	*(void**)(&powmImpl_2048) = dlsym(dlHandle, "powm_2048");
	if ((error = dlerror()) != NULL) {
		return error;
	}

	return NULL;
}

// Unloading the library invalidates all the loaded function pointers
// To make debugging more obvious, they're set to NULL before the dynamic library is closed
char *unloadLibrary() {
	char *error;
	if (dlHandle != NULL) {
		// null out function pointers first
		powmImpl_2048 = NULL;
		// clear dlerror
		dlerror();
		dlclose(dlhandle);
		if ((error = dlerror()) != NULL) {
			return error;
		}
		return NULL;
	} else {
		return "Cannot unload library that hasn't been loaded";
	}
}

// Actually run the modular exponentiation on the GPU based on the loaded library
struct powm_return_t *powm_2048(const void *prime, const void *instances, const uint32_t len) {
	return (*powmImpl_2048)(prime, instances, len);
}

*/
import "C"
import (
	"errors"
	"fmt"
	"gitlab.com/elixxir/crypto/cyclic"
	"gitlab.com/elixxir/crypto/large"
)

func main() {
	// Not sure what q would be for MODP2048, so leaving it at 1
	const bitLen = 2048
	g := cyclic.NewGroup(
		large.NewIntFromString("FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3DC2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F83655D23DCA3AD961C62F356208552BB9ED529077096966D670C354E4ABC9804F1746C08CA18217C32905E462E36CE3BE39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9DE2BCBF6955817183995497CEA956AE515D2261898FA051015728E5A8AACAA68FFFFFFFFFFFFFFFF", 16),
		large.NewInt(2),
		large.NewInt(1),
	)
	// x**y mod p
	x := g.NewIntFromString("102698389601429893247415098320984", 10)
	y := g.NewIntFromString("8891261048623689650221543816983486", 10)
	pMem := C.CBytes(g.GetP().CGBNMem(bitLen))
	result := g.Exp(x, y, g.NewInt(2))
	fmt.Printf("result in Go: %v\n", result.TextVerbose(16, 0))
	// x**y mod p: x (2048-4096 bits)
	// For more than one X and Y, they would be appended in the list
	var cgbnInputs []byte
	cgbnInputs = append(cgbnInputs, x.CGBNMem(bitLen)...)
	cgbnInputs = append(cgbnInputs, y.CGBNMem(bitLen)...)
	inputsMem := C.CBytes(cgbnInputs)
	errorString, err := C.loadLibrary()
	if err != nil {
		fmt.Printf("%v\n", err.Error())
		return
	}
	// At some point we will also need to free the error string, right?
	// Pain in the ass, tbh
	if errorString != nil {
		// TODO Try something: how does C.GoString behave when the char* is NULL?
		//  Hopefully it would return an empty string, and not crash
		errorStringGo := C.GoString(errorString)
		if errorStringGo != "" {
			fmt.Printf("%v\n", errorStringGo)
			return
		}
	}
	cudaResult := C.powm_2048(pMem, inputsMem, 1)
	if cudaResult != nil {
		errorString := C.GoString(cudaResult.error)
		if errorString != "" {
			panic(errors.New(errorString))
		}
		resultBytes := C.GoBytes(cudaResult.outputs, bitLen/8)
		// The results are in that byte string!
		// So we just need to get them into a big int.
		resultInt := g.NewIntFromCGBN(resultBytes)
		fmt.Printf("result in Go from CUDA: %v\n", resultInt.TextVerbose(16, 0))
	}
	errorString = C.unloadLibrary()
	if errorString != nil {
		err := C.GoString(errorString)
		if err != "" {
			panic(errors.New(err))
		}
	}
}
