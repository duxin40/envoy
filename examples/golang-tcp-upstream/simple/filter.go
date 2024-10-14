package main

import (
	"encoding/binary"
	"strings"

	"dubbo.apache.org/dubbo-go/v3/common/constant"
	"dubbo.apache.org/dubbo-go/v3/filter/generic/generalizer"
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"

	hessian "github.com/apache/dubbo-go-hessian2"
)

type tcpUpstreamFilter struct {
	api.EmptyTcpUpstreamFilter

	callbacks api.TcpUpstreamCallbackHandler
	config    *config
}

func (f *tcpUpstreamFilter) EncodeData(buffer api.BufferInstance, endOfStream bool) bool {
	api.LogInfof("[tcpUpstreamFilter][EncodeData] route: %+v", f.callbacks.StreamInfo().GetRouteName())
	clusterName, _ := f.callbacks.StreamInfo().VirtualClusterName()
	api.LogInfof("[tcpUpstreamFilter][EncodeData] cluster: %+v", clusterName)
	api.LogInfof("[tcpUpstreamFilter][EncodeData]req buffer: %+v", buffer.String())
	api.LogInfof("[tcpUpstreamFilter][EncodeData] config: %+v", f.config)
	f.callbacks.EnableHalfClose(true)

	return false
}

const (
	DUBBO_LENGTH_OFFSET = 12
	DUBBO_MAGIC_SIZE    = 2
	DUBBO_HEADER_SIZE   = 16
)

func (f *tcpUpstreamFilter) OnUpstreamData(buffer api.BufferInstance, endOfStream bool) api.UpstreamDataStatus {
	api.LogInfof("[tcpUpstreamFilter][OnUpstreamData]resp buffer len: %+v", buffer.Len())
	if buffer.Len() < DUBBO_MAGIC_SIZE || binary.BigEndian.Uint16(buffer.Bytes()) != hessian.MAGIC {
		api.LogInfof("[tcpUpstreamFilter][OnUpstreamData] Magic error, buffer.Len(): %+v", buffer.Len())
		return api.UpstreamDataFailure
	}
	if buffer.Len() < hessian.HEADER_LENGTH {
		api.LogInfof("[tcpUpstreamFilter][OnUpstreamData] Header length error, buffer.Len(): %+v", buffer.Len())
		return api.UpstreamDataFailure
	}
	bodyLength := binary.BigEndian.Uint32(buffer.Bytes()[DUBBO_LENGTH_OFFSET:])
	if buffer.Len() < (int(bodyLength) + hessian.HEADER_LENGTH) {
		api.LogInfof("[tcpUpstreamFilter][OnUpstreamData] NeedMoreData for Body, buffer.Len(): %+v", buffer.Len())
		return api.UpstreamDataContinue
	}

	api.LogInfof("[tcpUpstreamFilter][OnUpstreamData] end, length: %+v", buffer.Len())

	return api.UpstreamDataFinish
}

func (*tcpUpstreamFilter) OnDestroy(reason api.DestroyReason) {
	api.LogInfof("golang-test [http2rpc][OnDestroy] , reason: %+v", reason)
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
