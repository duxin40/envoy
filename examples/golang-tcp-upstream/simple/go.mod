module github.com/envoyproxy/envoy/examples/golang-network/simple

// the version should >= 1.18
go 1.22

toolchain go1.22.0

// NOTICE: these lines could be generated automatically by "go mod tidy"
require (
	github.com/cncf/xds/go v0.0.0-20240905190251-b4127c9b8d78
	github.com/envoyproxy/envoy v1.31.2
	google.golang.org/protobuf v1.35.1
)

require (
	dubbo.apache.org/dubbo-go/v3 v3.1.1
	github.com/apache/dubbo-go-hessian2 v1.12.4
)

require (
	cel.dev/expr v0.15.0 // indirect
	github.com/dubbogo/gost v1.14.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.0.4 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/natefinch/lumberjack v2.0.0+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	go.uber.org/zap v1.21.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240102182953-50ed04b92917 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a12d6c2237 // indirect
)

// TODO: remove when #26173 lands.
replace github.com/envoyproxy/envoy => ../../..
