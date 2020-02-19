module gitlab.seznam.net/sklik-devops/slo-exporter

go 1.13

require (
	github.com/DATA-DOG/go-sqlmock v1.4.1
	github.com/asaskevich/govalidator v0.0.0-20200108200545-475eaeb16496
	github.com/go-kit/kit v0.9.0
	github.com/go-test/deep v1.0.5
	github.com/gorilla/mux v1.7.3
	github.com/grafana/loki v6.7.8+incompatible
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hpcloud/tail v1.0.0
	github.com/lib/pq v1.3.0
	github.com/prometheus/client_golang v1.3.0
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/viper v1.6.2
	github.com/stretchr/testify v1.4.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/fsnotify.v1 v1.4.7 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.2.4
)
