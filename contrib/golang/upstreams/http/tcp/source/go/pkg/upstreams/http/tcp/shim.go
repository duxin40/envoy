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
	"sync"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

var (
	ErrDupRequestKey = errors.New("dup request key")
)

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

	err := Requests.StoreReq(r, req)
	if err != nil {
		panic(fmt.Sprintf("createRequest failed, err: %s", err.Error()))
	}

	configId := uint64(r.configId)

	filterFactory, config := getHttpTcpBridgeFactoryAndConfig(req.pluginName(), configId)
	f := filterFactory(config, req)
	req.httpTcpBridge = f

	return req
}

func getRequest(r *C.httpRequest) *httpRequest {
	return Requests.GetReq(r)
}

//export envoyGoHttpTcpBridgeOnEncodeHeader
func envoyGoHttpTcpBridgeOnEncodeHeader(s *C.processState, endStream, headerNum, headerBytes, buffer, length uint64) uint64 {
	state := getOrCreateState(s)
	defer state.RecoverPanic()

	req := state.request
	buf := &httpBuffer{
		state:               state,
		envoyBufferInstance: buffer,
		length:              length,
	}

	filter := req.httpTcpBridge
	header := &requestHeaderMapImpl{
		requestOrResponseHeaderMapImpl{
			headerMapImpl{
				state:       state,
				headerNum:   headerNum,
				headerBytes: headerBytes,
			},
		},
	}
	return uint64(filter.EncodeHeaders(header, buf, endStream == uint64(api.EndStream)))
}

//export envoyGoHttpTcpBridgeOnEncodeData
func envoyGoHttpTcpBridgeOnEncodeData(s *C.processState, endStream, buffer, length uint64) uint64 {
	state := getOrCreateState(s)
	defer state.RecoverPanic()

	req := state.request
	buf := &httpBuffer{
		state:               state,
		envoyBufferInstance: buffer,
		length:              length,
	}

	filter := req.httpTcpBridge

	return uint64(filter.EncodeData(buf, endStream == uint64(api.EndStream)))
}

//export envoyGoHttpTcpBridgeOnUpstreamData
func envoyGoHttpTcpBridgeOnUpstreamData(s *C.processState, endStream, headerNum, headerBytes, buffer, length uint64) uint64 {

	state := getOrCreateState(s)
	defer state.RecoverPanic()

	req := state.request
	buf := &httpBuffer{
		state:               state,
		envoyBufferInstance: buffer,
		length:              length,
	}

	filter := req.httpTcpBridge
	header := &responseHeaderMapImpl{
		requestOrResponseHeaderMapImpl{
			headerMapImpl{
				state:       state,
				headerNum:   headerNum,
				headerBytes: headerBytes,
			},
		},
	}

	return uint64(filter.OnUpstreamData(header, buf, endStream == uint64(api.EndStream)))
}

//export envoyGoHttpTcpBridgeOnDestroy
func envoyGoHttpTcpBridgeOnDestroy(r *C.httpRequest) {
	req := getRequest(r)
	// do nothing even when get panic, since filter is already destroying.
	defer req.recoverPanic()

	f := req.httpTcpBridge
	f.OnDestroy()

	req.httpTcpBridge = nil

	Requests.DeleteReq(r)
}
