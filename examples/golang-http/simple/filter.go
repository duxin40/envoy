package main

import (
	"fmt"
	// "bytes"
	"strconv"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	// "dubbo.apache.org/dubbo-go/v3/protocol/dubbo/impl"

	// "cgw.cestc.cn/gateway-control-plane/pkg/plugins/stream"
	"dubbo.apache.org/dubbo-go/v3/common/constant"
	"dubbo.apache.org/dubbo-go/v3/filter/generic/generalizer"
	// "dubbo.apache.org/dubbo-go/v3/protocol"
	dubbo2 "dubbo.apache.org/dubbo-go/v3/protocol/dubbo"
	invocation2 "dubbo.apache.org/dubbo-go/v3/protocol/invocation"
	"dubbo.apache.org/dubbo-go/v3/remoting"
	"encoding/binary"
	// "encoding/json"
	hessian "github.com/apache/dubbo-go-hessian2"
	// "github.com/dubbogo/gost/log/logger"
	// "mosn.io/htnn/api/pkg/filtermanager/api"
	"strings"
)

var UpdateUpstreamBody = "upstream response body updated by the simple plugin"

// The callbacks in the filter, like `DecodeHeaders`, can be implemented on demand.
// Because api.PassThroughStreamFilter provides a default implementation.
type filter struct {
	api.PassThroughStreamFilter

	Header   api.RequestHeaderMap
	RespHeader api.ResponseHeaderMap

	callbacks api.FilterCallbackHandler
	path      string
	config    *config
}

// func (f *filter) sendLocalReplyInternal() api.StatusType {
// 	body := fmt.Sprintf("%s, path: %s\r\n", f.config.echoBody, f.path)
// 	f.callbacks.DecoderFilterCallbacks().SendLocalReply(200, body, nil, 0, "")
// 	// Remember to return LocalReply when the request is replied locally
// 	return api.LocalReply
// }

// Callbacks which are called in request path
// The endStream is true if the request doesn't have body
func (f *filter) DecodeHeaders(header api.RequestHeaderMap, endStream bool) api.StatusType {
	api.LogInfo("[http2rpc][DecodeHeaders] start")
	f.Header = header
	header.Set(":status", "200")
	//headers.Set(":method", "CONNECT")

	api.LogInfo("[http2rpc][DecodeHeaders] end")
	return api.StopAndBuffer
}

// DecodeData might be called multiple times during handling the request body.
// The endStream is true when handling the last piece of the body.
func (f *filter) DecodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	api.LogInfo("[http2rpc][DecodeData] start")

	mtdname := "sayName"
	oldargs := map[string]interface{}{
		"name": "jackduxinxxx",
	}

	types := make([]string, 0, len(oldargs))
	args := make([]hessian.Object, 0, len(oldargs))
	attchments := map[string]interface{}{
		constant.GenericKey:   constant.GenericSerializationDefault,
		constant.InterfaceKey: "com.alibaba.nacos.example.dubbo.service.DemoService",
		constant.MethodKey:    mtdname,
	}

	g := getGeneralizer(constant.GenericSerializationDefault)

	for _, arg := range oldargs {
		// use the default generalizer(MapGeneralizer)
		typ, err := g.GetType(arg)
		if err != nil {
			api.LogErrorf("failed to get type, %v", err)
		}
		obj, err := g.Generalize(arg)
		if err != nil {
			api.LogErrorf("generalization failed, %v", err)
			return api.Continue
		}
		types = append(types, typ)
		args = append(args, obj)
	}

	// construct a new invocation for generic call
	newArgs := []interface{}{
		mtdname,
		types,
		args,
	}
	newIvc := invocation2.NewRPCInvocation(constant.Generic, newArgs, attchments)
	//newIvc.SetReply(genericInvocation.Reply())
	//newIvc.Attachments()[constant.GenericKey] = invoker.GetURL().GetParam(constant.GenericKey, "")
	newIvc.SetAttachment(constant.PathKey, "com.alibaba.nacos.example.dubbo.service.DemoService")
	newIvc.SetAttachment(constant.InterfaceKey, "com.alibaba.nacos.example.dubbo.service.DemoService")
	newIvc.SetAttachment(constant.VersionKey, "1.0.0")
	//newIvc.SetAttachment(constant.GroupKey, "DEFAULT_GROUP")
	//newIvc.SetAttachment(constant.ServiceKey, "demoService")
	api.LogInfo(fmt.Sprintf("newIvc: %+v", newIvc))

	codec := &dubbo2.DubboCodec{}
	req := remoting.NewRequest("2.0.2")

	req.ID = 1
	rsp := remoting.NewPendingResponse(req.ID)
	rsp.Reply = newIvc.Reply()
	remoting.AddPendingResponse(rsp)

	req.Data = newIvc
	req.Event = false
	req.TwoWay = true
	buf, err := codec.EncodeRequest(req)
	if err != nil {
		api.LogErrorf("failed to encode request, req: %+v, buf: %+v, err: %+v", req.Data, buf, err)
		return api.LocalReply
		// return &api.LocalResponse{
		// 	Code:   500,
		// 	Msg:    "failed to encode dubbo request",
		// 	Header: nil,
		// }
	}

	//api.LogInfof("[http2rpc][DecodeRequest] req: %+v", buf.String())
	_ = buffer.Set(buf.Bytes())

	api.LogInfo("[http2rpc][DecodeData] end")
	return api.Continue
}

func (f *filter) DecodeTrailers(trailers api.RequestTrailerMap) api.StatusType {
	// support suspending & resuming the filter in a background goroutine
	return api.Continue
}

// Callbacks which are called in response path
// The endStream is true if the response doesn't have body
func (f *filter) EncodeHeaders(header api.ResponseHeaderMap, endStream bool) api.StatusType {
	api.LogInfo("[http2rpc][EncodeHeaders] start")
	header.Set("Content-Type", "application/json; charset=utf-8")
	header.Set(":status", "300")
	// header.Set("Content-Length", "87")
	f.RespHeader = header
	if endStream {
		return api.Continue
	}
	api.LogInfo("[http2rpc][EncodeHeaders] end")
	// return api.StopAndBuffer
	return api.StopAndBuffer
}

const (
	DUBBO_LENGTH_OFFSET = 12
	DUBBO_MAGIC_SIZE    = 2
	DUBBO_HEADER_SIZE   = 16
)

// EncodeData might be called multiple times during handling the response body.
// The endStream is true when handling the last piece of the body.
func (f *filter) EncodeData(buffer api.BufferInstance, endStream bool) api.StatusType {
	api.LogInfo("[http2rpc][EncodeData] start")
	if buffer.Len() < DUBBO_MAGIC_SIZE || binary.BigEndian.Uint16(buffer.Bytes()) != hessian.MAGIC {
		//_ = data.Set([]byte(hessian.ErrIllegalPackage.Error()))
		api.LogInfof("[http2rpc][EncodeData] Magic error, buffer.Len(): %+v", buffer.Len())
		return api.StopAndBuffer
	}
	if buffer.Len() < hessian.HEADER_LENGTH {
		api.LogInfof("[http2rpc][EncodeData] Header length error, buffer.Len(): %+v", buffer.Len())
		return api.StopAndBuffer
	}
	bodyLength := binary.BigEndian.Uint32(buffer.Bytes()[DUBBO_LENGTH_OFFSET:])
	if buffer.Len() < (int(bodyLength) + hessian.HEADER_LENGTH) {
		api.LogInfof("[http2rpc][EncodeData] Body length error, buffer.Len(): %+v", buffer.Len())
		return api.StopAndBuffer
	}
	api.LogInfof("[http2rpc][EncodeData] data: %+v", buffer)
	api.LogInfof("[http2rpc][EncodeData] data: %+v", string(buffer.Bytes()[DUBBO_HEADER_SIZE :]))

	// 读取Dubbo消息的长度
	// dubboDataLength := int(binary.BigEndian.Uint32(buffer.Bytes()[DUBBO_LENGTH_OFFSET :]))
	// // data.Drain(DUBBO_HEADER_SIZE)
	// dubboBody := buffer.Bytes()[DUBBO_HEADER_SIZE : DUBBO_HEADER_SIZE+dubboDataLength]
	// api.LogInfof("[http2rpc][EncodeData] dubboBody: %+v", string(dubboBody))

	//data.Drain(16)

	// // 开始正事
	// codec := &dubbo2.DubboCodec{}
	// result, _, err := codec.Decode(buffer.Bytes())
	// api.LogInfof("[http2rpc][EncodeResponse] codec.Decode, err: %+v", err)
	// r := result.Result.(*remoting.Response)
	// jsonBytes, _ := json.Marshal(r.Result.(protocol.RPCResult))
	// _ = buffer.Set(jsonBytes)

	// buf := bytes.NewBuffer(buffer.Bytes())
	// pkg := impl.NewDubboPackage(buf)
	// err := pkg.Unmarshal()
	// if err != nil {
	// 	api.LogInfof("[http2rpc][EncodeResponse] Unmarshal, err: %+v", err)
	// }

	b := buffer.Bytes()[DUBBO_HEADER_SIZE:]
	decoder := hessian.NewDecoder(b)
	_, err := decoder.Decode()
	if err != nil {
		panic(fmt.Sprintf("[http2rpc][EncodeResponse] Decode, err: %+v", err))
	}
	rsp, err := decoder.Decode()
	if err != nil {
		panic(fmt.Sprintf("[http2rpc][EncodeResponse] Decode-2, err: %+v", err))
	}
	api.LogInfof("[http2rpc][EncodeResponse] Decode, val: %+v", rsp)
	bodyBytes := []byte(fmt.Sprintf("%s", rsp))
	_ = buffer.Set(bodyBytes)
	// f.RespHeader.Set("Content-Length", buffer.Len())

	api.LogInfof("[http2rpc][EncodeData] end, length: %+v", buffer.Len())
	return api.Continue
}

func (f *filter) EncodeTrailers(trailers api.ResponseTrailerMap) api.StatusType {
	return api.Continue
}

// OnLog is called when the HTTP stream is ended on HTTP Connection Manager filter.
func (f *filter) OnLog() {
	code, _ := f.callbacks.StreamInfo().ResponseCode()
	respCode := strconv.Itoa(int(code))
	api.LogDebug(respCode)

	/*
		// It's possible to kick off a goroutine here.
		// But it's unsafe to access the f.callbacks because the FilterCallbackHandler
		// may be already released when the goroutine is scheduled.
		go func() {
			defer func() {
				if p := recover(); p != nil {
					const size = 64 << 10
					buf := make([]byte, size)
					buf = buf[:runtime.Stack(buf, false)]
					fmt.Printf("http: panic serving: %v\n%s", p, buf)
				}
			}()

			// do time-consuming jobs
		}()
	*/
}

// OnLogDownstreamStart is called when HTTP Connection Manager filter receives a new HTTP request
// (required the corresponding access log type is enabled)
func (f *filter) OnLogDownstreamStart() {
	// also support kicking off a goroutine here, like OnLog.
}

// OnLogDownstreamPeriodic is called on any HTTP Connection Manager periodic log record
// (required the corresponding access log type is enabled)
func (f *filter) OnLogDownstreamPeriodic() {
	// also support kicking off a goroutine here, like OnLog.
}

func (f *filter) OnDestroy(reason api.DestroyReason) {
	// One should not access f.callbacks here because the FilterCallbackHandler
	// is released. But we can still access other Go fields in the filter f.

	// goroutine can be used everywhere.
}

func getGeneralizer(generic string) (g generalizer.Generalizer) {
	switch strings.ToLower(generic) {
	case constant.GenericSerializationDefault:
		g = generalizer.GetMapGeneralizer()
	case constant.GenericSerializationGson:
		g = generalizer.GetGsonGeneralizer()

	default:
		api.LogInfof("\"%s\" is not supported, use the default generalizer(MapGeneralizer)", generic)
		g = generalizer.GetMapGeneralizer()
	}
	return
}
