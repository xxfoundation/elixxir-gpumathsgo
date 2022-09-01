////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

//+build !linux !gpu

package gpumaths

// api_cpu.go (and all of the *_cpu.go files) hold stub information necessary
// to make the api build without the importers having to do anything.
// Instead of crashing/breaking the build, we return error messages back
// on all the api calls in lieu of performing the operation in the cpu (this
// is future work)

// NoGpuErrStr is the error returned when the gpu is not supported inthe build.
const NoGpuErrStr = "gpumaths stubbed build doesn't support CUDA stream pool"
