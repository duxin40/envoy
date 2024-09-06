#include "upstream_request.h"

#include <cstdint>
#include <memory>
#include <utility>

#include "envoy/upstream/upstream.h"

#include "source/common/common/assert.h"
#include "source/common/common/utility.h"
#include "source/common/http/codes.h"
#include "source/common/http/header_map_impl.h"
#include "source/common/http/headers.h"
#include "source/common/http/message_impl.h"
#include "source/common/network/transport_socket_options_impl.h"
#include "source/common/router/router.h"
#include "source/extensions/common/proxy_protocol/proxy_protocol_header.h"

namespace Envoy {
namespace Extensions {
namespace Upstreams {
namespace Http {
namespace Tcp {
namespace Golang {

void TcpConnPool::onPoolReady(Envoy::Tcp::ConnectionPool::ConnectionDataPtr&& conn_data,
                              Upstream::HostDescriptionConstSharedPtr host) {
  upstream_handle_ = nullptr;
  wrapper_ = new TcpConnPoolWrapper(shared_from_this());
  Network::Connection& latched_conn = conn_data->connection();
  auto upstream =
      std::make_shared<TcpUpstream>(&callbacks_->upstreamToDownstream(), std::move(conn_data), dynamic_lib_, wrapper_);

  go_conn_id_ = dynamic_lib_->envoyGoOnUpstreamConnectionReady(wrapper_,
   reinterpret_cast<unsigned long long>(plugin_name_.data()), plugin_name_.length(),
      config_id_);

  ENVOY_LOG(debug, "get host info: {}", host->cluster().name());

  callbacks_->onPoolReady(upstream, host, latched_conn.connectionInfoProvider(),
                          latched_conn.streamInfo(), {});      
  // callbacks_->onPoolReady(std::move(upstream), host, latched_conn.connectionInfoProvider(),
                          // latched_conn.streamInfo(), {});      
}

TcpUpstream::TcpUpstream(Router::UpstreamToDownstream* upstream_request,
                         Envoy::Tcp::ConnectionPool::ConnectionDataPtr&& upstream, Dso::TcpUpstreamDsoPtr dynamic_lib, TcpConnPoolWrapper* wrapper)
    : upstream_request_(upstream_request), upstream_conn_data_(std::move(upstream)), dynamic_lib_(dynamic_lib), wrapper_(wrapper) {
  upstream_conn_data_->connection().enableHalfClose(true);
  upstream_conn_data_->addUpstreamCallbacks(*this);
}

void TcpUpstream::encodeData(Buffer::Instance& data, bool end_stream) {
  // end_stream = false;

  Buffer::RawSliceVector slice_vector = data.getRawSlices();
  int slice_num = slice_vector.size();
  unsigned long long* slices = new unsigned long long[2 * slice_num];
  for (int i = 0; i < slice_num; i++) {
    const Buffer::RawSlice& s = slice_vector[i];
    slices[2 * i] = reinterpret_cast<unsigned long long>(s.mem_);
    slices[2 * i + 1] = s.len_;
  }

  GoUint64 if_end_stream = dynamic_lib_->envoyGoEncodeData(wrapper_, data.length(),
  reinterpret_cast<GoUint64>(slices), slice_num, end_stream);
  if (if_end_stream == 0) {
    end_stream = false;
  } else {
    end_stream = true;
  }

  upstream_conn_data_->connection().write(data, end_stream);
}

Envoy::Http::Status TcpUpstream::encodeHeaders(const Envoy::Http::RequestHeaderMap& headers,
                                               bool end_stream) {
   
  ENVOY_LOG(debug, "encodeHeaders: {}", headers);

  // Headers should only happen once, so use this opportunity to add the proxy
  // proto header, if configured.
  const Router::RouteEntry* route_entry = upstream_request_->route().routeEntry();
  
  ASSERT(route_entry != nullptr);
  if (route_entry->connectConfig().has_value()) {
    Buffer::OwnedImpl data;
    const auto& connect_config = route_entry->connectConfig();
    if (connect_config->has_proxy_protocol_config() &&
        upstream_request_->connection().has_value()) {
      Extensions::Common::ProxyProtocol::generateProxyProtoHeader(
          connect_config->proxy_protocol_config(), *upstream_request_->connection(), data);
    }

    if (data.length() != 0 || end_stream) {
      upstream_conn_data_->connection().write(data, end_stream);
    }
  }

  // TcpUpstream::encodeHeaders is called after the UpstreamRequest is fully initialized. Alsoc use
  // this time to synthesize the 200 response headers downstream to complete the CONNECT handshake.
  Envoy::Http::ResponseHeaderMapPtr headersToDownstream{
      Envoy::Http::createHeaderMap<Envoy::Http::ResponseHeaderMapImpl>(
          {{Envoy::Http::Headers::get().Status, "200"}})};
  upstream_request_->decodeHeaders(std::move(headersToDownstream), false);
  return Envoy::Http::okStatus();
}

void TcpUpstream::encodeTrailers(const Envoy::Http::RequestTrailerMap&) {
  Buffer::OwnedImpl data;
  upstream_conn_data_->connection().write(data, true);
}

void TcpUpstream::enableHalfClose() {
  upstream_conn_data_->connection().enableHalfClose(true);
}

void TcpUpstream::readDisable(bool disable) {
  if (upstream_conn_data_->connection().state() != Network::Connection::State::Open) {
    return;
  }
  upstream_conn_data_->connection().readDisable(disable);
}

void TcpUpstream::resetStream() {
  upstream_request_ = nullptr;
  upstream_conn_data_->connection().close(Network::ConnectionCloseType::NoFlush);
}

void TcpUpstream::onUpstreamData(Buffer::Instance& data, bool end_stream) {
  ENVOY_LOG(debug, "onUpstreamData, data: {}, end: {}", data.toString(), end_stream);

  Buffer::RawSliceVector slice_vector = data.getRawSlices();
  int slice_num = slice_vector.size();
  unsigned long long* slices = new unsigned long long[2 * slice_num];
  for (int i = 0; i < slice_num; i++) {
    const Buffer::RawSlice& s = slice_vector[i];
    slices[2 * i] = reinterpret_cast<unsigned long long>(s.mem_);
    slices[2 * i + 1] = s.len_;
  }

  GoUint64 status = dynamic_lib_->envoyGoOnUpstreamData(wrapper_, data.length(),
  reinterpret_cast<GoUint64>(slices), slice_num, end_stream);
  if (status == 0) { // UpstreamDataContinue 
    return;
  } else if (status == 1) {
    end_stream = true;
    upstream_request_->decodeData(data, end_stream);
    return;
  } else {
    end_stream = true;
    data.drain(data.length());
    data.add(ProtocolErrorMessage);
    upstream_request_->decodeData(data, end_stream);
    return;
  }

  // ENVOY_LOG(debug, "onUpstreamData, end_stream: {}", end_stream);
  // // end_stream = true;
  // if (response_buffer_.length() == 0) {
  //   if (data.length() < DUBBO_MAGIC_SIZE || data.peekBEInt<uint16_t>() != DUBBO_MAGIC_NUMBER) {
  //     data.drain(data.length());
  //     data.add(ProtocolErrorMessage);
  //     upstream_request_->decodeData(data, end_stream);
  //     return;
  //   }
  // }
  // if (decodeDubboFrame(data) == DubboFrameDecodeStatus::Ok) {
  //   uint32_t body_length_ = response_buffer_.peekBEInt<uint32_t>(DUBBO_LENGTH_OFFSET);
  //   data.move(response_buffer_, body_length_ + DUBBO_HEADER_SIZE);
  //   upstream_request_->decodeData(data, end_stream);
  // }
}

DubboFrameDecodeStatus TcpUpstream::decodeDubboFrame(Buffer::Instance& data) {
  response_buffer_.move(data);
  if (response_buffer_.length() < DUBBO_HEADER_SIZE) {
    return DubboFrameDecodeStatus::NeedMoreData;
  }

  uint32_t body_length_ = response_buffer_.peekBEInt<uint32_t>(DUBBO_LENGTH_OFFSET);
  if (response_buffer_.length() < body_length_ + DUBBO_HEADER_SIZE) {
    return DubboFrameDecodeStatus::NeedMoreData;
  }

  return DubboFrameDecodeStatus::Ok;
}

void TcpUpstream::onEvent(Network::ConnectionEvent event) {
  dynamic_lib_->envoyGoOnUpstreamEvent(wrapper_, static_cast<int>(event));

  if (event != Network::ConnectionEvent::Connected && upstream_request_) {
    upstream_request_->onResetStream(Envoy::Http::StreamResetReason::ConnectionTermination, "");
  }
}

void TcpUpstream::onAboveWriteBufferHighWatermark() {
  if (upstream_request_) {
    upstream_request_->onAboveWriteBufferHighWatermark();
  }
}

void TcpUpstream::onBelowWriteBufferLowWatermark() {
  if (upstream_request_) {
    upstream_request_->onBelowWriteBufferLowWatermark();
  }
}

void TcpUpstream::enableHalfClose(bool enabled) {
  // if (closed_) {
  //   ENVOY_LOG(warn, "connection has closed, addr: {}", addr_);
  //   return;
  // }
  ASSERT(upstream_conn_data_ != nullptr);
  upstream_conn_data_->connection().enableHalfClose(enabled);
  ENVOY_LOG(debug, "set enableHalfClose, enabled: {}, actualEnabled: {}", enabled, upstream_conn_data_->connection().isHalfCloseEnabled());
}

} // namespace Golang
} // namespace Tcp
} // namespace Http
} // namespace Upstreams
} // namespace Extensions
} // namespace Envoy