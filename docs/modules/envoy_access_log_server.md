# Envoy access log server

|                |                        |
|----------------|------------------------|
| `moduleName`   | `envoyAccessLogServer` |
| Module type    | `producer`             |
| Output event   | `raw`                  |

This module allows you to generate events based on access logs sent form remote [Envoy proxy](https://www.envoyproxy.io/) over a gRPC interface.

### Envoy support and configuration
At this moment, V3 of envoy's xDS API is supported. [See the upstream documentation for details](https://www.envoyproxy.io/docs/envoy/latest/api/api_supported_versions) on API versions.

In particular, you have to configure your envoy instance to send access_logs using [v3 AccessLogService rpc](https://github.com/envoyproxy/envoy/blob/842485709d651a6057b2ffb505ffced21173e004/api/envoy/service/accesslog/v3/als.proto#L39). We currently do not implement handling of other versions.

See a minimal example on how to configure envoy to send access_logs to an slo-exporter instance (envoy v1.15+):

```yaml
static_resources:
  listeners:
  - name: listener_0
    address:
      socket_address:
        address: 0.0.0.0
        port_value: 8080
    filter_chains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: ingress_http
          access_log:
          - name: envoy.access_loggers.http_grpc
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.access_loggers.grpc.v3.HttpGrpcAccessLogConfig
              common_config:
                grpc_service:
                  envoy_grpc:
                    cluster_name: service_accesslog
                buffer_size_bytes:
                  value: 0
                log_name: accesslogv3
                transport_api_version: V3 # needed to ensure that v3.AccessLogService is used
              additional_request_headers_to_log: ['slo-result', 'slo-class', 'slo-app', 'slo-endpoint']
[...]
  clusters:
    - name: service_accesslog
      connect_timeout: 6s
      type: LOGICAL_DNS
      load_assignment:
        cluster_name: service_accesslog
        endpoints:
          - lb_endpoints:
              - endpoint:
                  address:
                    socket_address:
                      address: localhost # exporter host
                      port_value: 18090  # exporter port
      http2_protocol_options: {}
[...]
```

Full working example is available here: [`/examples/envoy_proxy/envoy/envoy.yaml`](/examples/envoy_proxy/envoy/envoy.yaml).

### Resulting event metadata
Please note that some of the keys may not be present	|

#### Common properties
| metadata's key                    |  example(s)            | description |
|-----------------------------------|------------------------|-------------|
| downstreamDirectRemoteAddress     | `77.75.74.172`, `2a02:598:3333:1::1` | IP address (v4 or v6) |
| downstreamDirectRemotePort	    | `443` | TCP port number |
| downstreamLocalAddress	        | `77.75.74.172`, `2a02:598:3333:1::1` | IP address (v4 or v6) |
| downstreamLocalPort               | `443` | TCP port number |
| downstreamRemoteAddress	        | `77.75.74.172`, `2a02:598:3333:1::1` | IP address (v4 or v6) |
| downstreamRemotePort	            | `443` | TCP port number |
| routeName                         | `fooRoute` | Name of the route as present in an envoy's configuration |
| sampleRate	                    | `1.0`, `0.0` | Indicates the rate at which this log entry was sampled. Valid range is (0.0, 1.0].
| startTime	                        | RFC3339 `2020-12-22T14:27:28Z` | The time that Envoy started servicing this request.
| timeToFirstDownstreamTxByte       | `32451342ns` | Interval between the first downstream byte received and the first downstream byte sent.
| timeToFirstUpstreamRxByte	        | `32451342ns` | Interval between the first downstream byte received and the first upstream byte received (i.e. time it takes to start receiving a response).
| timeToFirstUpstreamTxByte	        | `32451342ns` | Interval between the first downstream byte received and the first upstream byte sent. |
| timeToLastDownstreamTxByte        | `32451342ns` | Interval between the first downstream byte received and the last downstream byte sent. |
| timeToLastRxByte	                | `32451342ns` | Interval between the first downstream byte received and the last downstream byte received (i.e. time it takes to receive a request). |
| timeToLastUpstreamRxByte	        | `32451342ns` | Interval between the first downstream byte received and the last upstream byte received (i.e. time it takes to receive a complete response). |
| timeToLastUpstreamTxByte          | `32451342ns` | Interval between the first downstream byte received and the last upstream byte sent. |
| upstreamCluster                   | `fooUpstream` | Name of the upstream cluster as present in an envoy's configuration |
| upstreamLocalAddress              | `77.75.74.172`, `2a02:598:3333:1::1` | IP address (v4 or v6) |
| upstreamLocalPort                 | `443` | TCP port number |
| upstreamRemoteAddress	            | `77.75.74.172`, `2a02:598:3333:1::1` | IP address (v4 or v6) |
| upstreamRemotePort	            | `443` | TCP port number |
| upstreamTransportFailureReason    | "TLS handshake" | [%UPSTREAM_TRANSPORT_FAILURE_REASON%](https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/usage) |

*Note: please see [envoy documentation](https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/usage) on explanation on how *RemoteAddress,*ReportPort is filled.*

#### HTTP access_log request properties
| metadata's key                    |  example(s)            | description |
|-----------------------------------|------------------------|-------------|
| authority     | `neverssl.com`, `neverssl.com:80` | HTTP/2 `authority` or HTTP/1.1 `Host` header value. |
| forwardedFor  | `203.0.113.195, 70.41.3.18, 150.172.238.178` | X-Forwarded-For HTTP header |
| originalPath  | `/` | Value of the ``X-Envoy-Original-Path`` request header. |
| path          | `/` | The path portion from the incoming request URI. |
| referer       | `Referer: https://www.seznam.cz` | Value of the `Referer` request header. |
| http_*request_header_name* e.g. `http_slo-domain` | `userportal` | Request's HTTP header |
| requestBodyBytes      | `32` ||
| requestHeadersBytes   | `32` ||
| requestId             | `e087fb8b-ee2f-4d92-bb83-afdabc8cceee` | Value of the ``X-Request-Id`` request header |
| requestMethod         | `GET` | HTTP method name |
| scheme                | `http` | The scheme portion of the incoming request URI. |
| userAgent             | `curl/7.74.0-DEV` | Value of the `User-Agent` request header. |

#### HTTP access_log response properties
| metadata's key                    |  example(s)            | description |
|-----------------------------------|------------------------|-------------|
| responseBodyBytes | `32` ||
| responseCodeDetails | `via_upstream` | The HTTP response code details. |
| responseCode | `200` | HTTP response code |
| responseHeadersBytes | `32` ||
| sent_http_*response_header_name* (e.g. `sent_http_slo-domain`) | `userportal` | Response's HTTP header |
| sent_trailer_*trailer_name* (e.g. `sent_trailer_slo-result`)   | `success` | Response's HTTP trailer |

#### TCP access_log properties
| metadata's key                    |  example(s)            | description |
|-----------------------------------|------------------------|-------------|
| receivedBytes | `32` ||
| sentBytes | `32` ||

### moduleConfig
```yaml
# IP address and port for GRPC server to bind to. See [net.Listen](https://golang.org/pkg/net/#Listen) on details of TCP network's possible representation of an address.
address: ":18090"
# gracefulShutdownTimeout for the GRPC server. Please note also the existence of 'maximumGracefulShutdownDuration' global config option which is effectively an upper boundary of here-specified timeout value.
gracefulShutdownTimeout: "5s"
# eventIdMetadataKey it's value will be used as a unique id for the generated event if present.
eventIdMetadataKey: <string>
```
