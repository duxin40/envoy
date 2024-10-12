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
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

const (
	NoWaitingCallback  = 0
	MayWaitingCallback = 1
)

type panicInfo struct {
	paniced bool
	details string
}

type httpRequest struct {
	req             *C.httpRequest
	httpFilter      api.TcpUpstreamFilter
	pInfo           panicInfo
	waitingLock     sync.Mutex // protect waitingCallback
	cond            sync.Cond
	waitingCallback int32

	// protect multiple cases:
	// 1. protect req_->strValue in the C++ side from being used concurrently.
	// 2. protect waitingCallback from being modified in markMayWaitingCallback concurrently.
	mutex sync.Mutex

	// decodingState and encodingState are part of httpRequest, not another GC object.
	// So, no cycle reference, GC finalizer could work well.
	decodingState processState
	encodingState processState
	streamInfo    streamInfo
}

// processState implements the FilterCallbacks interface.
type processState struct {
	request      *httpRequest
	processState *C.processState
}

const (
	// Values align with "enum class FilterState" in C++
	ProcessingHeader  = 1
	ProcessingData    = 4
	ProcessingTrailer = 6
)

func (s *processState) Phase() api.EnvoyRequestPhase {
	if s.processState.is_encoding == 0 {
		switch int(s.processState.state) {
		case ProcessingHeader:
			return api.DecodeHeaderPhase
		case ProcessingData:
			return api.DecodeDataPhase
		case ProcessingTrailer:
			return api.DecodeTrailerPhase
		}
	}
	// s.processState.is_encoding == 1
	switch int(s.processState.state) {
	case ProcessingHeader:
		return api.EncodeHeaderPhase
	case ProcessingData:
		return api.EncodeDataPhase
	case ProcessingTrailer:
		return api.EncodeTrailerPhase
	}
	panic(fmt.Errorf("unexpected state, is_encoding: %d, state: %d", s.processState.is_encoding, s.processState.state))
}

func (s *processState) Continue(status api.StatusType) {
	panic("please implement me")
}

func (s *processState) SendLocalReply(responseCode int, bodyText string, headers map[string][]string, grpcStatus int64, details string) {
	panic("please implement me")
}

func (s *processState) sendPanicReply(details string) {
	panic("please implement me")
}

func (s *processState) RecoverPanic() {
	if e := recover(); e != nil {
		buf := debug.Stack()

		if e == errRequestFinished || e == errFilterDestroyed {
			api.LogInfof("http: panic serving: %v (Client may cancel the request prematurely)\n%s", e, buf)
		} else {
			api.LogErrorf("http: panic serving: %v\n%s", e, buf)
		}

		switch e {
		case errRequestFinished, errFilterDestroyed:
			// do nothing

		case errNotInGo:
			// We can not send local reply now, since not in go now,
			// will delay to the next time entering Go.
			s.request.pInfo = panicInfo{
				paniced: true,
				details: fmt.Sprint(e),
			}

		default:
			// The following safeReplyPanic should only may get errRequestFinished,
			// errFilterDestroyed or errNotInGo, won't hit this branch, so, won't dead loop here.

			// errInvalidPhase, or other panic, not from not-ok C return status.
			// It's safe to try send a local reply with 500 status.
			s.sendPanicReply(fmt.Sprint(e))
		}
	}
}

func (r *httpRequest) StreamInfo() api.StreamInfo {
	return &r.streamInfo
}

func (r *httpRequest) DecoderFilterCallbacks() api.DecoderFilterCallbacks {
	return &r.decodingState
}

func (r *httpRequest) EncoderFilterCallbacks() api.EncoderFilterCallbacks {
	return &r.encodingState
}

// markWaitingOnEnvoy marks the request may be waiting a callback from envoy.
// Must be the NoWaitingCallback state since it's invoked under the r.mutex lock.
// We do not do lock waitingCallback here, to reduce lock contention.
func (r *httpRequest) markMayWaitingCallback() {
	if !atomic.CompareAndSwapInt32(&r.waitingCallback, NoWaitingCallback, MayWaitingCallback) {
		panic("markWaitingCallback: unexpected state")
	}
}

// markNoWaitingOnEnvoy marks the request is not waiting a callback from envoy.
// Can not make sure it's in the MayWaitingCallback state, since the state maybe changed by OnDestroy.
func (r *httpRequest) markNoWaitingCallback() {
	atomic.StoreInt32(&r.waitingCallback, NoWaitingCallback)
}

// checkOrWaitCallback checks if we need to wait a callback from envoy, and wait it.
func (r *httpRequest) checkOrWaitCallback() {
	// need acquire the lock, since there might be concurrency race with resumeWaitCallback.
	r.cond.L.Lock()
	defer r.cond.L.Unlock()

	// callback or OnDestroy already called, no need to wait.
	if atomic.LoadInt32(&r.waitingCallback) == NoWaitingCallback {
		return
	}
	r.cond.Wait()
}

// resumeWaitCallback resumes the goroutine that waiting for the callback from envoy.
func (r *httpRequest) resumeWaitCallback() {
	// need acquire the lock, since there might be concurrency race with checkOrWaitCallback.
	r.cond.L.Lock()
	defer r.cond.L.Unlock()

	if atomic.CompareAndSwapInt32(&r.waitingCallback, MayWaitingCallback, NoWaitingCallback) {
		// Broadcast is safe even there is no waiters.
		r.cond.Broadcast()
	}
}

func (r *httpRequest) pluginName() string {
	return C.GoStringN(r.req.plugin_name.data, C.int(r.req.plugin_name.len))
}

// recover goroutine to stop Envoy process crashing when panic happens
func (r *httpRequest) recoverPanic() {
	if e := recover(); e != nil {
		buf := debug.Stack()

		if e == errRequestFinished || e == errFilterDestroyed {
			api.LogInfof("http: panic serving: %v (Client may cancel the request prematurely)\n%s", e, buf)
		} else {
			api.LogErrorf("http: panic serving: %v\n%s", e, buf)
		}
	}
}

func (r *httpRequest) ClearRouteCache() {
	panic("please implement me")
}

func (r *httpRequest) Continue(status api.StatusType) {
	panic("please implement me")
}

func (r *httpRequest) SendLocalReply(responseCode int, bodyText string, headers map[string][]string, grpcStatus int64, details string) {
	panic("please implement me")
}

func (r *httpRequest) Log(level api.LogType, message string) {
	// TODO performance optimization points:
	// Add a new goroutine to write logs asynchronously and avoid frequent cgo calls
	cAPI.HttpLog(level, fmt.Sprintf("[http][%v] %v", r.pluginName(), message))
	// The default log format is:
	// [2023-08-09 03:04:16.179][1390][error][golang] [contrib/golang/common/log/cgo.cc:24] [http][plugin_name] msg
}

func (r *httpRequest) LogLevel() api.LogType {
	return cAPI.HttpLogLevel()
}

func (r *httpRequest) GetProperty(key string) (string, error) {
	panic("please implement me")
}

func (r *httpRequest) Finalize(reason int) {
	cAPI.HttpFinalize(unsafe.Pointer(r), reason)
}

func (s *httpRequest) EnableHalfClose(enabled bool) {
	var enabledInt int
	if enabled {
		enabledInt = 1
	}
	cAPI.UpstreamConnEnableHalfClose(unsafe.Pointer(s), enabledInt)
}

type streamInfo struct {
	request *httpRequest
}

func (s *streamInfo) GetRouteName() string {
	name, _ := cAPI.HttpGetStringValue(unsafe.Pointer(s.request), ValueRouteName)
	return name
}

func (s *streamInfo) FilterChainName() string {
	panic("please implement me")
}

func (s *streamInfo) Protocol() (string, bool) {
	panic("please implement me")
}

func (s *streamInfo) ResponseCode() (uint32, bool) {
	panic("please implement me")
}

func (s *streamInfo) ResponseCodeDetails() (string, bool) {
	panic("please implement me")
}

func (s *streamInfo) AttemptCount() uint32 {
	panic("please implement me")
}

type dynamicMetadata struct {
	request *httpRequest
}

func (s *streamInfo) DynamicMetadata() api.DynamicMetadata {
	return &dynamicMetadata{
		request: s.request,
	}
}

func (d *dynamicMetadata) Get(filterName string) map[string]interface{} {
	panic("please implement me")
}

func (d *dynamicMetadata) Set(filterName string, key string, value interface{}) {
	panic("please implement me")
}

func (s *streamInfo) DownstreamLocalAddress() string {
	panic("please implement me")
}

func (s *streamInfo) DownstreamRemoteAddress() string {
	panic("please implement me")
}

// UpstreamLocalAddress return the upstream local address.
func (s *streamInfo) UpstreamLocalAddress() (string, bool) {
	panic("please implement me")
}

// UpstreamRemoteAddress return the upstream remote address.
func (s *streamInfo) UpstreamRemoteAddress() (string, bool) {
	panic("please implement me")
}

func (s *streamInfo) UpstreamClusterName() (string, bool) {
	panic("please implement me")
}

func (s *streamInfo) VirtualClusterName() (string, bool) {
	name, _ := cAPI.HttpGetStringValue(unsafe.Pointer(s.request), ValueClusterName)
	return name, true
}

func (s *streamInfo) WorkerID() uint32 {
	return uint32(s.request.req.worker_id)
}

type filterState struct {
	request *httpRequest
}

func (s *streamInfo) FilterState() api.FilterState {
	return &filterState{
		request: s.request,
	}
}

func (f *filterState) SetString(key, value string, stateType api.StateType, lifeSpan api.LifeSpan, streamSharing api.StreamSharing) {
	panic("please implement me")
}

func (f *filterState) GetString(key string) string {
	panic("please implement me")
}

type tcpUpstreamConfig struct {
	config *C.httpConfig
}

func (c *tcpUpstreamConfig) Finalize() {
	cAPI.HttpConfigFinalize(unsafe.Pointer(c.config))
}

func (c *tcpUpstreamConfig) DefineCounterMetric(name string) api.CounterMetric {
	panic("please implement me")
}

func (c *tcpUpstreamConfig) DefineGaugeMetric(name string) api.GaugeMetric {
	panic("please implement me")
}

type counterMetric struct {
	config   *tcpUpstreamConfig
	metricId uint32
}

func (m *counterMetric) Increment(offset int64) {
	panic("please implement me")
}

func (m *counterMetric) Get() uint64 {
	panic("please implement me")
}

func (m *counterMetric) Record(value uint64) {
	panic("please implement me")
}

type gaugeMetric struct {
	config   *tcpUpstreamConfig
	metricId uint32
}

func (m *gaugeMetric) Increment(offset int64) {
	panic("please implement me")
}

func (m *gaugeMetric) Get() uint64 {
	panic("please implement me")
}

func (m *gaugeMetric) Record(value uint64) {
	panic("please implement me")
}
