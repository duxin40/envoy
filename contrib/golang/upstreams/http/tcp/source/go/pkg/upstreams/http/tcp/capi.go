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

#cgo CFLAGS: -I../../../../../../common/go/api -I../api
#cgo linux LDFLAGS: -Wl,-unresolved-symbols=ignore-all
#cgo darwin LDFLAGS: -Wl,-undefined,dynamic_lookup

#include <stdlib.h>
#include <string.h>

#include "api.h"

*/
import "C"
import (
	"strings"
	"unsafe"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

var cgoAPI api.TcpUpstreamCAPI = &cgoApiImpl{}

func SetCgoAPI(apiImpl api.NetworkCAPI) {
	if apiImpl != nil {
		cgoAPI = apiImpl
	}
}

type cgoApiImpl struct{}

func (c *cgoApiImpl) UpstreamInfo(f unsafe.Pointer, infoType int) string {
	var info string
	C.envoyGoFilterUpstreamInfo(f, C.int(infoType), unsafe.Pointer(&info))
	return strings.Clone(info)
}
