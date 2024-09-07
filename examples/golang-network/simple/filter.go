package main

import (
	"fmt"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"

	"github.com/envoyproxy/envoy/contrib/golang/upstreams/http/tcp/source/go/pkg/upstreams/http/tcp"
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

func (*tcpUpstreamFilter) EncodeData(buffer []byte, endOfStream bool) bool {
	return false
}

func (*tcpUpstreamFilter) OnUpstreamData(buffer []byte, endOfStream bool) api.UpstreamDataStatus {
	return api.UpstreamDataFinish
}

func (*tcpUpstreamFilter) OnEvent(event api.ConnectionEvent) {}

func main() {}
