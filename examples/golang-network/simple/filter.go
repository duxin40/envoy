package main

import (
	"encoding/binary"
	"fmt"
	"strings"

	"dubbo.apache.org/dubbo-go/v3/common/constant"
	"dubbo.apache.org/dubbo-go/v3/filter/generic/generalizer"
	"dubbo.apache.org/dubbo-go/v3/remoting"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"

	"github.com/envoyproxy/envoy/contrib/golang/upstreams/http/tcp/source/go/pkg/upstreams/http/tcp"

	// "bytes"

	// "dubbo.apache.org/dubbo-go/v3/protocol/dubbo/impl"

	// "cgw.cestc.cn/gateway-control-plane/pkg/plugins/stream"

	// "dubbo.apache.org/dubbo-go/v3/protocol"

	dubbo2 "dubbo.apache.org/dubbo-go/v3/protocol/dubbo"
	invocation2 "dubbo.apache.org/dubbo-go/v3/protocol/invocation"

	// "encoding/json"
	hessian "github.com/apache/dubbo-go-hessian2"
	// "github.com/dubbogo/gost/log/logger"
	// "mosn.io/htnn/api/pkg/filtermanager/api"
)

func init() {
	tcp.RegisterTcpUpstreamConfigFactory("simple-network", cf)
}

var cf = &configFactory{}

type configFactory struct{}

func (f *configFactory) CreateFactoryFromConfig(config interface{}) tcp.FilterFactory {
	return &filterFactory{}
}

type filterFactory struct {
}

func (f *filterFactory) CreateFilter() api.TcpUpstreamFilter {
	return &tcpUpstreamFilter{}
}

type tcpUpstreamFilter struct {
	api.EmptyTcpUpstreamFilter

	cb api.ConnectionCallback
}

func (*tcpUpstreamFilter) OnPoolReady(cb api.ConnectionCallback) {
	clusterName, _ := cb.StreamInfo().UpstreamClusterName()
	fmt.Println("go-side get clusterName: %s", clusterName)
	fmt.Println("go-side get routeName: %s", cb.StreamInfo().GetRouteName())

	enableHalfClose := true
	cb.EnableHalfClose(enableHalfClose)
	fmt.Println("go-side set enableHalfClose: %+v", enableHalfClose)
}

func (*tcpUpstreamFilter) OnPoolFailure(poolFailureReason api.PoolFailureReason, transportFailureReason string) {
}

func (*tcpUpstreamFilter) EncodeData(buffer api.BufferInstance, endOfStream bool) bool {
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
			return false
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
		return false
		// return &api.LocalResponse{
		// 	Code:   500,
		// 	Msg:    "failed to encode dubbo request",
		// 	Header: nil,
		// }
	}

	//api.LogInfof("[http2rpc][DecodeRequest] req: %+v", buf.String())
	_ = buffer.Set(buf.Bytes())

	api.LogInfo("[http2rpc][DecodeData] end")

	return false
}

const (
	DUBBO_LENGTH_OFFSET = 12
	DUBBO_MAGIC_SIZE    = 2
	DUBBO_HEADER_SIZE   = 16
)

func (*tcpUpstreamFilter) OnUpstreamData(buffer api.BufferInstance, endOfStream bool) api.UpstreamDataStatus {
	api.LogInfo("[http2rpc][EncodeData] start")
	if buffer.Len() < DUBBO_MAGIC_SIZE || binary.BigEndian.Uint16(buffer.Bytes()) != hessian.MAGIC {
		//_ = data.Set([]byte(hessian.ErrIllegalPackage.Error()))
		api.LogInfof("[http2rpc][EncodeData] Magic error, buffer.Len(): %+v", buffer.Len())
		return api.UpstreamDataFinish
	}
	if buffer.Len() < hessian.HEADER_LENGTH {
		api.LogInfof("[http2rpc][EncodeData] Header length error, buffer.Len(): %+v", buffer.Len())
		return api.UpstreamDataFinish
	}
	bodyLength := binary.BigEndian.Uint32(buffer.Bytes()[DUBBO_LENGTH_OFFSET:])
	if buffer.Len() < (int(bodyLength) + hessian.HEADER_LENGTH) {
		api.LogInfof("[http2rpc][EncodeData] Body length error, buffer.Len(): %+v", buffer.Len())
		return api.UpstreamDataFinish
	}
	api.LogInfof("[http2rpc][EncodeData] data: %+v", buffer)
	api.LogInfof("[http2rpc][EncodeData] data: %+v", string(buffer.Bytes()[DUBBO_HEADER_SIZE:]))

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

	return api.UpstreamDataFinish
}

func (*tcpUpstreamFilter) OnEvent(event api.ConnectionEvent) {}

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

func main() {}
