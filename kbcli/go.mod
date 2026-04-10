module kanban/kbcli

go 1.22

require (
	comdigger/core v0.0.0
	github.com/elastic/go-elasticsearch/v8 v8.19.3
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/elastic/elastic-transport-go/v8 v8.8.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/lib/pq v1.12.3 // indirect
	go.opentelemetry.io/otel v1.28.0 // indirect
	go.opentelemetry.io/otel/metric v1.28.0 // indirect
	go.opentelemetry.io/otel/trace v1.28.0 // indirect
)

replace comdigger/core => ../core
