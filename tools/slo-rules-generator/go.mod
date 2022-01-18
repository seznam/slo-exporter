module github.com/seznam/slo-exporter/tools/slo-rules-generator

go 1.16

require (
	github.com/prometheus/common v0.30.0
	// We fetch the exact revision because of issue described at https://github.com/prometheus/prometheus/issues/6048#issuecomment-534549253
	github.com/prometheus/prometheus v1.8.2-0.20210914090109-37468d88dce8
	github.com/stretchr/testify v1.7.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
)
