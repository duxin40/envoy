#pragma once

#include "contrib/envoy/extensions/upstreams/http/dubbo_tcp/v3/tcp_connection_pool.pb.h"
#include "envoy/registry/registry.h"
#include "envoy/router/router.h"

namespace Envoy {
namespace Extensions {
namespace Upstreams {
namespace Http {
namespace DubboTcp {

/**
 * Config registration for the TcpConnPool. @see Router::GenericConnPoolFactory
 */
class DubboTcpGenericConnPoolFactory : public Router::GenericConnPoolFactory {
public:
  std::string name() const override { return "envoy.filters.connection_pools.http.dubbo_tcp"; }
  std::string category() const override { return "envoy.upstreams"; }
  Router::GenericConnPoolPtr
  createGenericConnPool(Upstream::ThreadLocalCluster& thread_local_cluster,
                        Router::GenericConnPoolFactory::UpstreamProtocol upstream_protocol,
                        Upstream::ResourcePriority priority,
                        absl::optional<Envoy::Http::Protocol> downstream_protocol,
                        Upstream::LoadBalancerContext* ctx) const override;
  // Router::GenericConnPoolPtr
  // createGenericConnPool(Upstream::ThreadLocalCluster& thread_local_cluster, bool is_connect,
  //                       const Router::RouteEntry& route_entry,
  //                       absl::optional<Envoy::Http::Protocol> downstream_protocol,
  //                       Upstream::LoadBalancerContext* ctx) const override;
  ProtobufTypes::MessagePtr createEmptyConfigProto() override {
    return std::make_unique<
        envoy::extensions::upstreams::http::dubbo_tcp::v3::DubboTcpConnectionPoolProto>();
  }
};

DECLARE_FACTORY(DubboTcpGenericConnPoolFactory);

} // namespace DubboTcp
} // namespace Http
} // namespace Upstreams
} // namespace Extensions
} // namespace Envoy
