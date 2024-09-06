#include "config.h"

#include "upstream_request.h"

namespace Envoy {
namespace Extensions {
namespace Upstreams {
namespace Http {
namespace DubboTcp {

Router::GenericConnPoolPtr DubboTcpGenericConnPoolFactory::createGenericConnPool(
    Upstream::ThreadLocalCluster& thread_local_cluster,
    Router::GenericConnPoolFactory::UpstreamProtocol, Upstream::ResourcePriority priority,
    absl::optional<Envoy::Http::Protocol>, Upstream::LoadBalancerContext* ctx, const Protobuf::Message&) const {
  auto ret = std::make_shared<TcpConnPool>(thread_local_cluster, priority, ctx);
  return (ret->valid() ? std::move(ret) : nullptr);
}

// Router::GenericConnPoolPtr DubboTcpGenericConnPoolFactory::createGenericConnPool(
//     Upstream::ThreadLocalCluster& thread_local_cluster, bool is_connect,
//     const Router::RouteEntry& route_entry,
//     absl::optional<Envoy::Http::Protocol> downstream_protocol,
//     Upstream::LoadBalancerContext* ctx) const {
//   auto ret = std::make_unique<TcpConnPool>(thread_local_cluster, is_connect, route_entry,
//                                            downstream_protocol, ctx);
//   return (ret->valid() ? std::move(ret) : nullptr);
// }

REGISTER_FACTORY(DubboTcpGenericConnPoolFactory, Router::GenericConnPoolFactory);

} // namespace DubboTcp
} // namespace Http
} // namespace Upstreams
} // namespace Extensions
} // namespace Envoy
