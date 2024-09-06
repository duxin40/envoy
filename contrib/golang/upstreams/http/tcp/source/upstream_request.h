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
    // const auto* configPtr = dynamic_cast<const envoy::extensions::upstreams::http::tcp::golang::v3alpha::Config*>(&config);  

    envoy::extensions::upstreams::http::tcp::golang::v3alpha::Config c;
    // // MessageUtil::wireCast(config, c);
    // // bool x = any.Is<envoy::extensions::upstreams::http::tcp::golang::v3alpha::Config>();
    // // Protobuf::Message m = envoy::extensions::upstreams::http::tcp::golang::v3alpha::Config();

    // ENVOY_LOG(debug, "translate success config type name: {}", config.GetTypeName());
    // ENVOY_LOG(debug, "translate success config: {}", config.InitializationErrorString());
    
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

    // ProtobufWkt::Any any;
    // MessageUtil::packFrom(any, config);
    // ENVOY_LOG(debug, "translate success any: {}", any.DebugString());
    // // bool p = any.UnpackTo(c);
    // ENVOY_LOG(debug, "translate success any value: {}", any.value().c_str());

    // // auto* any_message = Protobuf::DynamicCastToGenerated<ProtobufWkt::Any>(&config);
    // // std::unique_ptr<Protobuf::Message> inner_message;
    // // inner_message = Helper::typeUrlToMessage(any_message->type_url());
    // // target_type_url = any_message->type_url();
    // // // inner_message must be valid as parsing would have already failed to load if there was an
    // // // invalid type_url.
    // // RETURN_IF_NOT_OK(MessageUtil::unpackTo(*any_message, *inner_message));



    // // absl::Status p = MessageUtil::unpackTo(any, c);
    // // ENVOY_LOG(debug, "translate success result: {}", p);

    // // absl::Status s = MessageUtil::unpackTo(any, c);

    // // bool s = any.UnpackTo(&c);

    // // std::string buf;
    // // auto res = any.SerializeToString(&buf);
    // // ENVOY_LOG(debug, "translate success SerializeToString: {}", res);


    // // ProtobufWkt::Any any_message;
    // // any_message.set_type_url("type.googleapis.com/envoy.extensions.upstreams.http.tcp.golang.v3alpha.Config");
    // // any_message.set_value(any.value());
    // // ENVOY_LOG(debug, "translate success any_message: {}", any_message.value());

    // // xds::type::v3::TypedStruct xx;
    // envoy::config::core::v3::TypedExtensionConfig xx;
    // bool ss = xx.ParseFromString(*any.mutable_value());
    // // absl::Status dd = MessageUtil::unpackTo(xx.typed_config(), c);
    // ENVOY_LOG(debug, "translate success result xxxxxxxxxx: {}", ss);
    // // ENVOY_LOG(debug, "translate success result xxxxxxxxxx: {}", dd);
    // ENVOY_LOG(debug, "translate success result: {}", xx.typed_config().value().c_str());
    // ENVOY_LOG(debug, "translate success result: {}", xx.typed_config().Is<envoy::extensions::upstreams::http::tcp::golang::v3alpha::Config>());
    // // ENVOY_LOG(debug, "translate success result: {}", c.library_id());





    // bool s = c.ParseFromString(any.value());
    // ENVOY_LOG(debug, "translate success type name: {}", config.GetTypeName());
    // ENVOY_LOG(debug, "translate success type name: {}", c.GetTypeName());
    // ENVOY_LOG(debug, "translate success type name: {}", any.GetTypeName());
    // ENVOY_LOG(debug, "translate success result: {}", s);
    // ENVOY_LOG(debug, "translate success result: {}", c.DebugString());
    // ENVOY_LOG(debug, "translate success result: {}", c.has_plugin_config());

    

    // // ENVOY_LOG(debug, "translate success: {}", config.GetDescriptor()->DebugString());
    // // ENVOY_LOG(debug, "translate success: {}", c.GetDescriptor()->DebugString());
    // ENVOY_LOG(debug, "translate success: {}", c.ByteSizeLong());
    // // ENVOY_LOG(debug, "translate success: {}", c.library_path().size());
    // // ENVOY_LOG(debug, "translate success: {}", c.library_path());
    // // ENVOY_LOG(debug, "translate success: {}", c.library_id().size());
    // // ENVOY_LOG(debug, "translate success: {}", c.library_id());
    // // ENVOY_LOG(debug, "translate success: {}", c.plugin_name().size());
    // // ENVOY_LOG(debug, "translate success: {}", c.plugin_name());
    // // ENVOY_LOG(debug, "translate success: {}", c.plugin_config().ByteSizeLong());
    // // ENVOY_LOG(debug, "translate success: {}", c.plugin_config().DebugString());
    // // ENVOY_LOG(debug, "translate success: {}", c.ByteSizeLong());

    // ENVOY_LOG(debug, "load tcp_upstream_golang library: {} {}", dynamic_lib_);

    // FilterConfigSharedPtr config = std::make_shared<FilterConfig>(
    //     c, dso_lib, fmt::format("{}tcp_upstream_golang.", stats_prefix), context);
    // config->newGoPluginConfig();
    // return [config, dso_lib](Http::FilterChainFactoryCallbacks& callbacks) {
    //   const std::string& worker_name = callbacks.dispatcher().name();
    //   auto pos = worker_name.find_first_of('_');
    //   ENVOY_BUG(pos != std::string::npos, "worker name is not in expected format worker_{id}");
    //   uint32_t worker_id;
    //   if (!absl::SimpleAtoi(worker_name.substr(pos + 1), &worker_id)) {
    //     IS_ENVOY_BUG("failed to parse worker id from name");
    //   }
    //   auto filter = std::make_shared<Filter>(config, dso_lib, worker_id);
    //   }

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

  // uint64_t wrapper_;
  Dso::TcpUpstreamDsoPtr dynamic_lib_;
  TcpConnPoolWrapper* wrapper_{nullptr};
};

class TcpUpstream : public Router::GenericUpstream,
                    public Envoy::Tcp::ConnectionPool::UpstreamCallbacks,
                    public std::enable_shared_from_this<TcpUpstream>,
                    Logger::Loggable<Logger::Id::golang>  {
public:
  TcpUpstream(Router::UpstreamToDownstream* upstream_request,
              Envoy::Tcp::ConnectionPool::ConnectionDataPtr&& upstream, Dso::TcpUpstreamDsoPtr dynamic_lib, TcpConnPoolWrapper* wrapper);

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

private:
  DubboFrameDecodeStatus decodeDubboFrame(Buffer::Instance& data);

private:
  Router::UpstreamToDownstream* upstream_request_;
  Envoy::Tcp::ConnectionPool::ConnectionDataPtr upstream_conn_data_;
  Buffer::OwnedImpl response_buffer_{};
  StreamInfo::BytesMeterSharedPtr bytes_meter_{std::make_shared<StreamInfo::BytesMeter>()};

  Dso::TcpUpstreamDsoPtr dynamic_lib_;
  TcpConnPoolWrapper* wrapper_{nullptr};
};

using TcpConPoolSharedPtr = std::shared_ptr<TcpConnPool>;
using TcpConnPoolWeakPtr = std::weak_ptr<TcpConnPool>;

struct TcpConnPoolWrapper {
public:
  TcpConnPoolWrapper(TcpConPoolSharedPtr ptr) : tcp_conn_pool_ptr_(ptr) {}
  ~TcpConnPoolWrapper() = default;

  TcpConPoolSharedPtr tcp_conn_pool_ptr_{};
  // anchor a string temporarily, make sure it won't be freed before copied to Go.
  std::string str_value_;
};

// using TcpUpstreamSharedPtr = std::shared_ptr<TcpUpstream>;
// using TcpUpstreamWeakPtr = std::weak_ptr<TcpUpstream>;

// struct TcpUpstreamWrapper {
// public:
//   TcpUpstreamWrapper(TcpUpstreamWeakPtr ptr) : tcp_upstream_ptr_(ptr) {}
//   ~TcpUpstreamWrapper() = default;

//   TcpUpstreamWeakPtr tcp_upstream_ptr_{};
//   // anchor a string temporarily, make sure it won't be freed before copied to Go.
//   std::string str_value_;
// };


} // namespace Golang
} // namespace Tcp
} // namespace Http
} // namespace Upstreams
} // namespace Extensions
} // namespace Envoy
