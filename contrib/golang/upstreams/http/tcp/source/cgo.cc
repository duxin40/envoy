// #include "contrib/golang/filters/network/source/golang.h"
#include "contrib/golang/upstreams/http/tcp/source/upstream_request.h"

namespace Envoy {
namespace Extensions {
namespace Upstreams {
namespace Http {
namespace Tcp {
namespace Golang {

//
// These functions may be invoked in another go thread,
// which means may introduce race between go thread and envoy thread.
// So we use the envoy's dispatcher in the filter to post it, and make it only executes in the envoy
// thread.
//

// Deep copy GoString into std::string, including the string content,
// it's safe to use it after the current cgo call returns.
std::string copyGoString(void* str) {
  if (str == nullptr) {
    return "";
  }
  auto go_str = reinterpret_cast<GoString*>(str);
  return std::string{go_str->p, size_t(go_str->n)};
}

// The returned absl::string_view only refer to the GoString, won't copy the string content into
// C++, should not use it after the current cgo call returns.
absl::string_view referGoString(void* str) {
  if (str == nullptr) {
    return "";
  }
  auto go_str = reinterpret_cast<GoString*>(str);
  return {go_str->p, static_cast<size_t>(go_str->n)};
}

enum class ConnectionInfoType {
  ConnectionInfoRouterName = 2,
	ConnectionInfoClusterName = 3,
};

extern "C" {

CAPIStatus envoyGoTcpUpstreamInfo(void* u, int info_type, void* ret) {
  auto* wrapper = reinterpret_cast<TcpConnPoolWrapper*>(u);
  TcpConPoolSharedPtr& shared_ptr = wrapper->tcp_conn_pool_ptr_;

  // if (TcpConPoolSharedPtr uu = weak_ptr.lock()) {
  auto* goStr = reinterpret_cast<GoString*>(ret);
  switch (static_cast<ConnectionInfoType>(info_type)) {
  case ConnectionInfoType::ConnectionInfoRouterName:
    wrapper->str_value_ = "test";
    break;
  case ConnectionInfoType::ConnectionInfoClusterName:
    wrapper->str_value_ = shared_ptr->host()->cluster().name();
    break;
  default:
    PANIC_DUE_TO_CORRUPT_ENUM;
  }

  goStr->p = wrapper->str_value_.data();
  goStr->n = wrapper->str_value_.length();
  return CAPIStatus::CAPIOK;
  // }
  
  return CAPIStatus::CAPIFilterIsGone;
}

CAPIStatus envoyGoTcpUpstreamConnEnableHalfClose(void* , int ) {
  // auto* wrapper = reinterpret_cast<TcpConnPoolWrapper*>(u);
  // TcpConPoolSharedPtr& shared_ptr = wrapper->tcp_conn_pool_ptr_;

  // shared_ptr->enableHalfClose(static_cast<bool>(enable_half_close));

  return CAPIOK;
}


} // extern "C"
} // namespace Golang
} // namespace Tcp
} // namespace Http
} // namespace Upstreams
} // namespace Extensions
} // namespace Envoy
