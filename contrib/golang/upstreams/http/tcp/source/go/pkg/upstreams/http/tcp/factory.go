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

import (
	"sync"

	"google.golang.org/protobuf/types/known/anypb"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type ConfigFactory interface {
	CreateFactoryFromConfig(config interface{}) FilterFactory
}

type FilterFactory interface {
	CreateFilter() api.TcpUpstreamFilter
}

type ConfigParser interface {
	// TODO: should return error when the config is invalid.
	ParseConfig(any *anypb.Any) interface{}
}

var (
	tcpUpstreamConfigFactoryMap = &sync.Map{} // pluginName -> ConfigFactory
)

func RegisterTcpUpstreamConfigFactory(name string, factory ConfigFactory) {
	if factory != nil {
		tcpUpstreamConfigFactoryMap.Store(name, factory)
	}
}

func GetTcpUpstreamConfigFactory(name string) ConfigFactory {
	if v, ok := tcpUpstreamConfigFactoryMap.Load(name); ok {
		return v.(ConfigFactory)
	}
	return nil
}

var tcpUpstreamConfigParser ConfigParser = &noopConfigParser{}

func RegisterTcpUpstreamConfigParser(parser ConfigParser) {
	if parser != nil {
		tcpUpstreamConfigParser = parser
	}
}

func GetTcpUpstreamConfigParser() ConfigParser {
	return tcpUpstreamConfigParser
}

type noopConfigParser struct{}

var _ ConfigParser = (*noopConfigParser)(nil)

func (n *noopConfigParser) ParseConfig(any *anypb.Any) interface{} {
	return any
}
