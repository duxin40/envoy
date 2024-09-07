#pragma once

#include <cstdint>
#include <memory>
#include <string>

#include "envoy/config/core/v3/extension.pb.h"
#include "envoy/http/codec.h"
#include "envoy/tcp/conn_pool.h"
#include "envoy/upstream/thread_local_cluster.h"

#include "google/protobuf/any.pb.h"
#include "source/common/buffer/watermark_buffer.h"
#include "source/common/common/cleanup.h"
#include "source/common/common/logger.h"
#include "source/common/config/well_known_names.h"
#include "source/common/router/upstream_request.h"
#include "source/common/stream_info/stream_info_impl.h"
#include "source/extensions/upstreams/http/tcp/upstream_request.h"

#include "contrib/golang/common/dso/dso.h"

#include "contrib/envoy/extensions/upstreams/http/tcp/golang/v3alpha/golang.pb.h"
#include "xds/type/v3/typed_struct.pb.h"

namespace Envoy {
namespace Extensions {
namespace Upstreams {
namespace Http {
namespace Tcp {
namespace Golang {

enum class DubboFrameDecodeStatus : uint8_t {
  Ok = 0,
  NeedMoreData = 1,
  InvalidHeader = 2,
};

constexpr uint64_t DUBBO_HEADER_SIZE = 16;
constexpr uint64_t DUBBO_MAGIC_SIZE = 2;
constexpr uint16_t DUBBO_MAGIC_NUMBER = 0xdabb;
constexpr uint64_t DUBBO_LENGTH_OFFSET = 12;
constexpr absl::string_view ProtocolErrorMessage = "Not dubbo message";

struct TcpConnPoolWrapper;

struct TcpUpstreamWrapper;

class TcpConnPool : public Router::GenericConnPool,
                    public Envoy::Tcp::ConnectionPool::Callbacks,
                    public std::enable_shared_from_this<TcpConnPool>,
                    Logger::Loggable<Logger::Id::golang> {
public:
  TcpConnPool(Upstream::ThreadLocalCluster& thread_local_cluster,
              Upstream::ResourcePriority priority, Upstream::LoadBalancerContext* ctx, const Protobuf::Message& config) {
    conn_pool_data_ = thread_local_cluster.tcpConnPool(priority, ctx);

    envoy::extensions::upstreams::http::tcp::golang::v3alpha::Config c;
    
    ProtobufWkt::Any any;
    any.ParseFromString(config.SerializeAsString());
    ENVOY_LOG(debug, "get cluster any value: {}", any.value());

    auto s = c.ParseFromString(any.value());
    ASSERT(s, "any.value() ParseFromString should always successful");

    std::string config_str;
    auto res = c.plugin_config().SerializeToString(&config_str);
    ASSERT(res, "plugin_config SerializeToString should always successful");

    ENVOY_LOG(debug, "load tcp_upstream_golang library at parse config: {} {}", c.library_id(), c.library_path());

    // loads DSO store a static map and a open handles leak will occur when the filter gets loaded and
    // unloaded.
    // TODO: unload DSO when filter updated.
    auto dso_lib = Dso::DsoManager<Dso::TcpUpstreamDsoImpl>::load(c.library_id(), c.library_path(), c.plugin_name());
    if (dso_lib == nullptr) {
      throw EnvoyException(fmt::format("tcp_upstream_golang: load library failed: {} {}", c.library_id(), c.library_path()));
    };

    dynamic_lib_ = dso_lib;

    if (dynamic_lib_) {
      config_id_ = dynamic_lib_->envoyGoOnTcpUpstreamConfig(reinterpret_cast<unsigned long long>(c.library_id().data()), c.library_id().length(),
     reinterpret_cast<unsigned long long>(config_str.data()), config_str.length());
    }
    plugin_name_ = c.plugin_name();

  }
  void newStream(Router::GenericConnectionPoolCallbacks* callbacks) override {
    callbacks_ = callbacks;
    upstream_handle_ = conn_pool_data_.value().newConnection(*this);
  }

  bool cancelAnyPendingStream() override {
    if (upstream_handle_) {
      upstream_handle_->cancel(Envoy::Tcp::ConnectionPool::CancelPolicy::Default);
      upstream_handle_ = nullptr;
      return true;
    }
    return false;
  }
  Upstream::HostDescriptionConstSharedPtr host() const override {
    return conn_pool_data_.value().host();
  }

  bool valid() const override { return conn_pool_data_.has_value(); }

  // Tcp::ConnectionPool::Callbacks
  void onPoolFailure(ConnectionPool::PoolFailureReason reason,
                     absl::string_view transport_failure_reason,
                     Upstream::HostDescriptionConstSharedPtr host) override {
    upstream_handle_ = nullptr;
    callbacks_->onPoolFailure(reason, transport_failure_reason, host);

    dynamic_lib_->envoyGoOnUpstreamConnectionFailure(wrapper_,
    static_cast<int>(reason), go_conn_id_);         
  }

  void onPoolReady(Envoy::Tcp::ConnectionPool::ConnectionDataPtr&& conn_data,
                   Upstream::HostDescriptionConstSharedPtr host) override;

private:
  uint64_t config_id_;
  std::string plugin_name_{};
  uint64_t go_conn_id_;
  absl::optional<Envoy::Upstream::TcpPoolData> conn_pool_data_;
  Envoy::Tcp::ConnectionPool::Cancellable* upstream_handle_{};
  Router::GenericConnectionPoolCallbacks* callbacks_{};
  Envoy::Tcp::ConnectionPool::ConnectionDataPtr upstream_conn_data_;

  Dso::TcpUpstreamDsoPtr dynamic_lib_;
  TcpConnPoolWrapper* wrapper_{nullptr};
};

class TcpUpstream : public Router::GenericUpstream,
                    public Envoy::Tcp::ConnectionPool::UpstreamCallbacks,
                    public std::enable_shared_from_this<TcpUpstream>,
                    Logger::Loggable<Logger::Id::golang>  {
public:
  TcpUpstream(Router::UpstreamToDownstream* upstream_request,
              Envoy::Tcp::ConnectionPool::ConnectionDataPtr&& upstream, Dso::TcpUpstreamDsoPtr dynamic_lib);

  // GenericUpstream
  void encodeData(Buffer::Instance& data, bool end_stream) override;
  void encodeMetadata(const Envoy::Http::MetadataMapVector&) override {}
  Envoy::Http::Status encodeHeaders(const Envoy::Http::RequestHeaderMap& headers, bool end_stream) override;
  void encodeTrailers(const Envoy::Http::RequestTrailerMap&) override;
  void enableHalfClose() override;
  void readDisable(bool disable) override;
  void resetStream() override;
  void setAccount(Buffer::BufferMemoryAccountSharedPtr) override {}

  // Tcp::ConnectionPool::UpstreamCallbacks
  void onUpstreamData(Buffer::Instance& data, bool end_stream) override;
  void onEvent(Network::ConnectionEvent event) override;
  void onAboveWriteBufferHighWatermark() override;
  void onBelowWriteBufferLowWatermark() override;
  const StreamInfo::BytesMeterSharedPtr& bytesMeter() override { return bytes_meter_; }

  void enableHalfClose(bool enabled);

  const Router::RouteEntry* route_entry_;
  TcpConnPoolWrapper* wrapper_{nullptr};

private:
  DubboFrameDecodeStatus decodeDubboFrame(Buffer::Instance& data);

private:
  Router::UpstreamToDownstream* upstream_request_;
  Envoy::Tcp::ConnectionPool::ConnectionDataPtr upstream_conn_data_;
  Buffer::OwnedImpl response_buffer_{};
  StreamInfo::BytesMeterSharedPtr bytes_meter_{std::make_shared<StreamInfo::BytesMeter>()};

  Dso::TcpUpstreamDsoPtr dynamic_lib_;
};

using TcpConPoolSharedPtr = std::shared_ptr<TcpConnPool>;
using TcpUpstreamSharedPtr = std::shared_ptr<TcpUpstream>;

struct TcpConnPoolWrapper {
public:
  TcpConnPoolWrapper(TcpConPoolSharedPtr ptr, TcpUpstreamSharedPtr bptr) : tcp_conn_pool_ptr_(ptr), tcp_upstream_ptr_(bptr) {}
  ~TcpConnPoolWrapper() = default;

  TcpConPoolSharedPtr tcp_conn_pool_ptr_{};
  TcpUpstreamSharedPtr tcp_upstream_ptr_{};
  // anchor a string temporarily, make sure it won't be freed before copied to Go.
  std::string str_value_;
};


} // namespace Golang
} // namespace Tcp
} // namespace Http
} // namespace Upstreams
} // namespace Extensions
} // namespace Envoy
