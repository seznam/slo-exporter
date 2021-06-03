module github.com/seznam/slo-exporter

go 1.15

require (
	github.com/antonmedv/expr v1.8.9
	github.com/chavacava/garif v0.0.0-20210405164556-e8a0a408d6af // indirect
	github.com/envoyproxy/go-control-plane v0.9.8
	github.com/go-kit/kit v0.10.0
	github.com/go-test/deep v1.0.6
	github.com/golang/protobuf v1.4.2
	github.com/gorilla/mux v1.7.4
	github.com/grafana/loki v1.5.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.1-0.20191002090509-6af20e3a5340
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hpcloud/tail v1.0.1-0.20180514194441-a1dbeea552b7
	github.com/iancoleman/strcase v0.0.0-20191112232945-16388991a334
	github.com/mattn/go-runewidth v0.0.12 // indirect
	github.com/mgechev/revive v1.0.6 // indirect
	github.com/prometheus/client_golang v1.6.0
	github.com/prometheus/common v0.10.0
	github.com/prometheus/prometheus v1.8.2-0.20200213233353-b90be6f32a33
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/segmentio/kafka-go v0.4.11
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/viper v1.7.0
	github.com/stretchr/testify v1.7.0
	golang.org/x/exp v0.0.0-20200513190911-00229845015e
	golang.org/x/sys v0.0.0-20210423185535-09eb48e85fd7 // indirect
	gonum.org/v1/gonum v0.7.0
	google.golang.org/grpc v1.27.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v2 v2.3.0
)

// Taken from Loki project https://github.com/grafana/loki/blob/master/go.mod#L66

replace github.com/Azure/azure-sdk-for-go => github.com/Azure/azure-sdk-for-go v36.2.0+incompatible

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.0+incompatible

// Without this, it attempts to upgrade to v0.18.x which has some conflicts with upstream Prometheus.
// Also, v0.17.5 is chosen to be consistent with Thanos and more clear than using a commit hash.
replace k8s.io/client-go => k8s.io/client-go v0.17.5
