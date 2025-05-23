load(
    "//bazel:envoy_build_system.bzl",
    "envoy_cc_library",
    "envoy_package",
)

licenses(["notice"])  # Apache 2

envoy_package()

envoy_cc_library(
    name = "substitution_formatter_lib",
    srcs = [
        "http_formatter_context.cc",
        "substitution_formatter.cc",
    ],
    hdrs = [
        "http_formatter_context.h",
        "substitution_formatter.h",
    ],
    deps = [
        "//envoy/api:api_interface",
        "//envoy/formatter:substitution_formatter_interface",
        "//envoy/stream_info:stream_info_interface",
        "//source/common/common:utility_lib",
        "//source/common/http:header_map_lib",
        "//source/common/http:utility_lib",
        "//source/common/json:json_loader_lib",
        "//source/common/json:json_streamer_lib",
        "//source/common/json:json_utility_lib",
        "@com_google_absl//absl/strings:str_format",
        "@envoy_api//envoy/config/core/v3:pkg_cc_proto",
    ],
)

envoy_cc_library(
    name = "substitution_format_string_lib",
    srcs = ["substitution_format_string.cc"],
    hdrs = ["substitution_format_string.h"],
    deps = [
        ":substitution_formatter_lib",
        "//source/common/config:datasource_lib",
        "//source/common/config:utility_lib",
        "//source/common/protobuf",
        "//source/server:generic_factory_context_lib",
        "@envoy_api//envoy/config/core/v3:pkg_cc_proto",
    ],
)

envoy_cc_library(
    name = "substitution_format_utility_lib",
    srcs = ["substitution_format_utility.cc"],
    hdrs = ["substitution_format_utility.h"],
    deps = [
        "//envoy/api:api_interface",
        "//envoy/http:protocol_interface",
        "//source/common/api:os_sys_calls_lib",
        "//source/common/http:header_utility_lib",
        "//source/common/http:utility_lib",
        "//source/common/protobuf:utility_lib",
        "//source/common/runtime:runtime_features_lib",
        "//source/common/stream_info:utility_lib",
    ],
)

envoy_cc_library(
    name = "http_speicific_formatter_extension_lib",
    srcs = ["http_specific_formatter.cc"],
    hdrs = ["http_specific_formatter.h"],
    deps = [
        "//envoy/api:api_interface",
        "//envoy/formatter:substitution_formatter_interface",
        "//envoy/runtime:runtime_interface",
        "//envoy/stream_info:stream_info_interface",
        "//envoy/upstream:upstream_interface",
        "//source/common/common:assert_lib",
        "//source/common/common:utility_lib",
        "//source/common/formatter:substitution_format_utility_lib",
        "//source/common/grpc:common_lib",
        "//source/common/http:utility_lib",
        "//source/common/json:json_loader_lib",
        "//source/common/protobuf:message_validator_lib",
        "//source/common/stream_info:utility_lib",
    ],
    alwayslink = 1,  # has factory registration
)

envoy_cc_library(
    name = "stream_info_formatter_extension_lib",
    srcs = ["stream_info_formatter.cc"],
    hdrs = ["stream_info_formatter.h"],
    rbe_pool = "6gig",
    deps = [
        "//envoy/api:api_interface",
        "//envoy/formatter:substitution_formatter_interface",
        "//envoy/runtime:runtime_interface",
        "//envoy/stream_info:stream_info_interface",
        "//envoy/upstream:upstream_interface",
        "//source/common/common:assert_lib",
        "//source/common/common:random_generator_lib",
        "//source/common/common:utility_lib",
        "//source/common/formatter:substitution_format_utility_lib",
        "//source/common/grpc:common_lib",
        "//source/common/http:utility_lib",
        "//source/common/json:json_loader_lib",
        "//source/common/json:json_utility_lib",
        "//source/common/protobuf:message_validator_lib",
        "//source/common/stream_info:utility_lib",
    ],
    alwayslink = 1,  # has factory registration
)

envoy_cc_library(
    name = "formatter_extension_lib",
    deps = [
        ":http_speicific_formatter_extension_lib",
        ":stream_info_formatter_extension_lib",
    ],
    alwayslink = 1,  # has factory registration
)
