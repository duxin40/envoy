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

 #cgo CFLAGS: -I../../../../../../../../../../../common/go/api -I../api
 #cgo linux LDFLAGS: -Wl,-unresolved-symbols=ignore-all
 #cgo darwin LDFLAGS: -Wl,-undefined,dynamic_lookup

 #include <stdlib.h>
 #include <string.h>

 #include "api.h"
*/
import "C"
import (
	"errors"
	"fmt"
	"runtime"
	"sync"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

var (
	// ref: https://golang.org/cmd/cgo/
	// The size of any C type T is available as C.sizeof_T, as in C.sizeof_struct_stat.
	CULLSize uintptr = C.sizeof_ulonglong

	ErrDupRequestKey = errors.New("dup request key")

	UpstreamFilters = &TcpUpstreamFilterMap{}

	upstreamConnIDGenerator uint64

	libraryID string

	initialized      = true
	envoyConcurrency uint32
)

// wrap the UpstreamFilter to ensure that the runtime.finalizer can be triggered
// regardless of whether there is a circular reference in the UpstreamFilter.
type upstreamConnWrapper struct {
	api.TcpUpstreamFilter
	finalizer *int
}

var Requests = &requestMap{}

type requestMap struct {
	requests sync.Map // *C.httpRequest -> *httpRequest
}

func (f *requestMap) StoreReq(key *C.httpRequest, req *httpRequest) error {
	if _, loaded := f.requests.LoadOrStore(key, req); loaded {
		return ErrDupRequestKey
	}
	return nil
}

func (f *requestMap) GetReq(key *C.httpRequest) *httpRequest {
	if v, ok := f.requests.Load(key); ok {
		return v.(*httpRequest)
	}
	return nil
}

func (f *requestMap) DeleteReq(key *C.httpRequest) {
	f.requests.Delete(key)
}

func (f *requestMap) Clear() {
	f.requests.Range(func(key, _ interface{}) bool {
		f.requests.Delete(key)
		return true
	})
}

func requestFinalize(r *httpRequest) {
	r.Finalize(api.NormalFinalize)
}

func getOrCreateState(s *C.processState) *processState {
	r := s.req
	req := getRequest(r)
	if req == nil {
		req = createRequest(r)
	}
	if s.is_encoding == 0 {
		if req.decodingState.processState == nil {
			req.decodingState.processState = s
		}
		return &req.decodingState
	}

	// s.is_encoding == 1
	if req.encodingState.processState == nil {
		req.encodingState.processState = s
	}
	return &req.encodingState
}

func createRequest(r *C.httpRequest) *httpRequest {
	req := &httpRequest{
		req: r,
	}
	req.decodingState.request = req
	req.encodingState.request = req
	req.streamInfo.request = req

	req.cond.L = &req.waitingLock
	// NP: make sure filter will be deleted.
	runtime.SetFinalizer(req, requestFinalize)

	err := Requests.StoreReq(r, req)
	if err != nil {
		panic(fmt.Sprintf("createRequest failed, err: %s", err.Error()))
	}

	configId := uint64(r.configId)

	filterFactory, config := getHttpFilterFactoryAndConfig(req.pluginName(), configId)
	f := filterFactory(config, req)
	req.httpFilter = f

	return req
}

func getRequest(r *C.httpRequest) *httpRequest {
	return Requests.GetReq(r)
}

func getState(s *C.processState) *processState {
	r := s.req
	req := getRequest(r)
	if s.is_encoding == 0 {
		return &req.decodingState
	}
	// s.is_encoding == 1
	return &req.encodingState
}

//export envoyGoEncodeData
func envoyGoEncodeData(s *C.processState, endStream, buffer, length uint64) uint64 {
	state := getOrCreateState(s)

	req := state.request

	// filter := UpstreamFilters.GetFilterByWrapper(uint64(uintptr(wrapper)))
	filter := req.httpFilter

	// isDecode := state.Phase() == api.DecodeDataPhase

	buf := &httpBuffer{
		state:               state,
		envoyBufferInstance: buffer,
		length:              length,
	}

	if filter.EncodeData(buf, endStream == 1) {
		return 1
	} else {
		return 0
	}
}

//export envoyGoOnUpstreamData
func envoyGoOnUpstreamData(s *C.processState, endStream, buffer, length uint64) uint64 {

	state := getOrCreateState(s)

	req := state.request

	// filter := UpstreamFilters.GetFilterByWrapper(uint64(uintptr(wrapper)))
	filter := req.httpFilter

	// isDecode := state.Phase() == api.DecodeDataPhase

	buf := &httpBuffer{
		state:               state,
		envoyBufferInstance: buffer,
		length:              length,
	}

	return uint64(filter.OnUpstreamData(buf, endStream == 1))
}

type TcpUpstreamFilterMap struct {
	idMap      sync.Map // upstreamConnID(uint) -> UpstreamFilter
	wrapperMap sync.Map // wrapper(uint64) -> UpstreamFilter
}

func (f *TcpUpstreamFilterMap) StoreFilterByConnID(key uint64, filter api.TcpUpstreamFilter) error {
	if _, loaded := f.idMap.LoadOrStore(key, filter); loaded {
		return ErrDupRequestKey
	}
	return nil
}

func (f *TcpUpstreamFilterMap) StoreFilterByWrapper(key uint64, filter api.TcpUpstreamFilter) error {
	if _, loaded := f.wrapperMap.LoadOrStore(key, filter); loaded {
		return ErrDupRequestKey
	}
	return nil
}

func (f *TcpUpstreamFilterMap) GetFilterByConnID(key uint64) api.TcpUpstreamFilter {
	if v, ok := f.idMap.Load(key); ok {
		return v.(api.TcpUpstreamFilter)
	}
	return nil
}

func (f *TcpUpstreamFilterMap) GetFilterByWrapper(key uint64) api.TcpUpstreamFilter {
	if v, ok := f.wrapperMap.Load(key); ok {
		return v.(api.TcpUpstreamFilter)
	}
	return nil
}

func (f *TcpUpstreamFilterMap) DeleteFilterByConnID(key uint64) {
	f.idMap.Delete(key)
}

func (f *TcpUpstreamFilterMap) DeleteFilterByWrapper(key uint64) {
	f.wrapperMap.Delete(key)
}

func (f *TcpUpstreamFilterMap) Clear() {
	f.idMap.Range(func(key, _ interface{}) bool {
		f.idMap.Delete(key)
		return true
	})
	f.wrapperMap.Range(func(key, _ interface{}) bool {
		f.wrapperMap.Delete(key)
		return true
	})
}
