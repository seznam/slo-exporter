module github.com/seznam/slo-exporter

go 1.16

require (
	github.com/envoyproxy/go-control-plane v0.9.9
	github.com/go-kit/kit v0.12.0
	github.com/go-test/deep v1.0.6
	github.com/golang/protobuf v1.5.2
	github.com/gorilla/mux v1.8.0
	github.com/grafana/loki v1.6.2-0.20211108122114-f61a4d2612d8
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.1-0.20191002090509-6af20e3a5340
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hpcloud/tail v1.0.1-0.20180514194441-a1dbeea552b7
	github.com/iancoleman/strcase v0.0.0-20191112232945-16388991a334
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.31.1
	github.com/prometheus/prometheus v1.8.2-0.20211011171444-354d8d2ecfac
	github.com/segmentio/kafka-go v0.4.11
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/viper v1.7.0
	github.com/stretchr/testify v1.7.0
	golang.org/x/exp v0.0.0-20200821190819-94841d0725da
	gonum.org/v1/gonum v0.8.2
	google.golang.org/grpc v1.40.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v2 v2.4.0
)

// Without this, it attempts to upgrade to v0.18.x which has some conflicts with upstream Prometheus.
// Also, v0.17.5 is chosen to be consistent with Thanos and more clear than using a commit hash.
replace k8s.io/client-go => k8s.io/client-go v0.17.5
