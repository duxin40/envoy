package main

import (

	// "net"

	// xds "github.com/cncf/xds/go/xds/type/v3"
	// "google.golang.org/protobuf/types/known/anypb"

	"fmt"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"

	"github.com/envoyproxy/envoy/contrib/golang/upstreams/http/tcp/source/go/pkg/upstreams/http/tcp"
)

func init() {
	// network.RegisterNetworkFilterConfigFactory("simple-network", cf)

	tcp.RegisterTcpUpstreamConfigFactory("simple-network", cf)
}

var cf = &configFactory{}

type configFactory struct{}

func (f *configFactory) CreateFactoryFromConfig(config interface{}) tcp.FilterFactory {
	// a := config.(*anypb.Any)
	// configStruct := &xds.TypedStruct{}
	// _ = a.UnmarshalTo(configStruct)

	// v := configStruct.Value.AsMap()["echo_server_addr"]
	// addr, err := net.LookupHost(v.(string))
	// if err != nil {
	//  fmt.Printf("fail to resolve: %v, err: %v\n", v.(string), err)
	//  return nil
	// }
	// upAddr := addr[0] + ":1025"

	// return &filterFactory{
	//  upAddr: upAddr,
	// }

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
	fmt.Println("#############")
	fmt.Println(cb.StreamInfo().UpstreamClusterName())
	fmt.Println("#############")
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

// func (f *tcpUpstreamFilter) OnNewConnection() api.FilterStatus {
// 	localAddr, _ := f.cb.StreamInfo().UpstreamLocalAddress()
// 	remoteAddr, _ := f.cb.StreamInfo().UpstreamRemoteAddress()
// 	fmt.Printf("[downFilter]OnNewConnection, local: %v, remote: %v, connect to: %v\n", localAddr, remoteAddr, f.upAddr)
// 	f.upFilter = &upFilter{
// 		downFilter: f,
// 		ch:         make(chan []byte, 1),
// 	}
// 	network.CreateUpstreamConn(f.upAddr, f.upFilter)
// 	return api.NetworkFilterContinue
// }

// func (f *downFilter) OnData(buffer []byte, endOfStream bool) api.UpstreamDataStatus {
// 	// remoteAddr, _ := f.cb.StreamInfo().UpstreamRemoteAddress()
// 	// fmt.Printf("[downFilter]OnData, addr: %v, buffer: %v, endOfStream: %v\n", remoteAddr, string(buffer), endOfStream)

// 	fmt.Printf("[downFilter]OnData, buffer: %v, endOfStream: %v\n", string(buffer), endOfStream)

// 	// buffer = append([]byte("hello, "), buffer...)
// 	// f.upFilter.ch <- buffer
// 	return api.UpstreamDataFinish
// }

// func (f *downFilter) OnEvent(event api.ConnectionEvent) {
// 	remoteAddr, _ := f.cb.StreamInfo().UpstreamRemoteAddress()
// 	fmt.Printf("[downFilter]OnEvent, addr: %v, event: %v\n", remoteAddr, event)
// }

// func (f *downFilter) OnWrite(buffer []byte, endOfStream bool) api.FilterStatus {
// 	fmt.Printf("[downFilter]OnWrite, buffer: %v, endOfStream: %v\n", string(buffer), endOfStream)
// 	return api.NetworkFilterContinue
// }

// type upFilter struct {
// 	api.EmptyUpstreamFilter

// 	cb         api.ConnectionCallback
// 	downFilter *downFilter
// 	ch         chan []byte
// }

// func (f *upFilter) OnPoolReady(cb api.ConnectionCallback) {
// 	f.cb = cb
// 	// f.cb.EnableHalfClose(false)
// 	localAddr, _ := f.cb.StreamInfo().UpstreamLocalAddress()
// 	remoteAddr, _ := f.cb.StreamInfo().UpstreamRemoteAddress()
// 	fmt.Printf("[upFilter]OnPoolReady, local: %v, remote: %v\n", localAddr, remoteAddr)
// 	// go func() {
// 	//  for {
// 	//    buf, ok := <-f.ch
// 	//    if !ok {
// 	//      return
// 	//    }
// 	//    f.cb.Write(buf, false)
// 	//  }
// 	// }()
// }

// func (f *upFilter) OnPoolFailure(poolFailureReason api.PoolFailureReason, transportFailureReason string) {
// 	fmt.Printf("[upFilter]OnPoolFailure, reason: %v, transportFailureReason: %v\n", poolFailureReason, transportFailureReason)
// }

// func (f *upFilter) OnData(buffer []byte, endOfStream bool) {
// 	remoteAddr, _ := f.cb.StreamInfo().UpstreamRemoteAddress()
// 	fmt.Printf("[upFilter]OnData, addr: %v, buffer: %v, endOfStream: %v\n", remoteAddr, string(buffer), endOfStream)
// 	// f.downFilter.cb.Write(buffer, endOfStream)
// }

// func (f *upFilter) OnEvent(event api.ConnectionEvent) {
// 	remoteAddr, _ := f.cb.StreamInfo().UpstreamRemoteAddress()
// 	fmt.Printf("[upFilter]OnEvent, addr: %v, event: %v\n", remoteAddr, event)
// 	// if event == api.LocalClose || event == api.RemoteClose {
// 	//  close(f.ch)
// 	// }
// }

func main() {}
