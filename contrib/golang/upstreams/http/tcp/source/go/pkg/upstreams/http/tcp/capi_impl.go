/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package tcp

/*
// ref https://github.com/golang/go/issues/25832

#cgo CFLAGS: -I../../../../../../../../common/go/api -I../api
#cgo linux LDFLAGS: -Wl,-unresolved-symbols=ignore-all
#cgo darwin LDFLAGS: -Wl,-undefined,dynamic_lookup

#include <stdlib.h>
#include <string.h>

#include "api.h"

*/
import "C"
import (
	"strings"
	"unsafe"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

var cAPI api.TcpUpstreamCAPI = &cgoApiImpl{}

func SetCgoAPI(apiImpl api.TcpUpstreamCAPI) {
	if apiImpl != nil {
		cAPI = apiImpl
	}
}

type cgoApiImpl struct{}

// When the status means unexpected stage when invoke C API,
// panic here and it will be recover in the Go entry function.
func handleCApiStatus(status C.CAPIStatus) {
	switch status {
	case C.CAPIFilterIsGone,
		C.CAPIFilterIsDestroy,
		C.CAPINotInGo,
		C.CAPIInvalidPhase:
		panic(capiStatusToStr(status))
	}
}

func capiStatusToStr(status C.CAPIStatus) string {
	switch status {
	case C.CAPIFilterIsGone:
		return errRequestFinished
	case C.CAPIFilterIsDestroy:
		return errFilterDestroyed
	case C.CAPINotInGo:
		return errNotInGo
	case C.CAPIInvalidPhase:
		return errInvalidPhase
	}

	return "unknown status"
}

func (c *cgoApiImpl) UpstreamInfo(f unsafe.Pointer, infoType int) string {
	var info string
	C.envoyGoTcpUpstreamInfo(f, C.int(infoType), unsafe.Pointer(&info))
	return strings.Clone(info)
}

func (c *cgoApiImpl) UpstreamConnEnableHalfClose(f unsafe.Pointer, enableHalfClose int) {
	C.envoyGoTcpUpstreamConnEnableHalfClose(f, C.int(enableHalfClose))
}

func (c *cgoApiImpl) HttpGetBuffer(s unsafe.Pointer, bufferPtr uint64, length uint64) []byte {
	state := (*processState)(s)
	buf := make([]byte, length)
	res := C.envoyGoTcpUpstreamGetBuffer(unsafe.Pointer(state.processState), C.uint64_t(bufferPtr), unsafe.Pointer(unsafe.SliceData(buf)))
	handleCApiStatus(res)
	return unsafe.Slice(unsafe.SliceData(buf), length)
}

func (c *cgoApiImpl) HttpDrainBuffer(s unsafe.Pointer, bufferPtr uint64, length uint64) {
	state := (*processState)(s)
	res := C.envoyGoTcpUpstreamDrainBuffer(unsafe.Pointer(state.processState), C.uint64_t(bufferPtr), C.uint64_t(length))
	handleCApiStatus(res)
}

func (c *cgoApiImpl) HttpSetBufferHelper(s unsafe.Pointer, bufferPtr uint64, value string, action api.BufferAction) {
	state := (*processState)(s)
	c.httpSetBufferHelper(state, bufferPtr, unsafe.Pointer(unsafe.StringData(value)), C.int(len(value)), action)
}

func (c *cgoApiImpl) HttpSetBytesBufferHelper(s unsafe.Pointer, bufferPtr uint64, value []byte, action api.BufferAction) {
	state := (*processState)(s)
	c.httpSetBufferHelper(state, bufferPtr, unsafe.Pointer(unsafe.SliceData(value)), C.int(len(value)), action)
}

func (c *cgoApiImpl) httpSetBufferHelper(state *processState, bufferPtr uint64, data unsafe.Pointer, length C.int, action api.BufferAction) {
	var act C.bufferAction
	switch action {
	case api.SetBuffer:
		act = C.Set
	case api.AppendBuffer:
		act = C.Append
	case api.PrependBuffer:
		act = C.Prepend
	}
	res := C.envoyGoTcpUpstreamSetBufferHelper(unsafe.Pointer(state.processState), C.uint64_t(bufferPtr), data, length, act)
	handleCApiStatus(res)
}
