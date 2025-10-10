module github.com/terraconstructs/grid/cmd/gridapi

go 1.24.0

toolchain go1.24.4

require (
	connectrpc.com/connect v1.19.0
	github.com/btcsuite/btcutil v1.0.2
	github.com/go-chi/chi/v5 v5.2.3
	github.com/go-chi/cors v1.2.1
	github.com/google/uuid v1.6.0
	github.com/spf13/cobra v1.10.1
	github.com/stretchr/testify v1.11.1
	github.com/terraconstructs/grid/api v0.0.0
	github.com/uptrace/bun v1.2.15
	github.com/uptrace/bun/dialect/pgdialect v1.2.15
	github.com/uptrace/bun/driver/pgdriver v1.2.15
	golang.org/x/net v0.44.0
	gonum.org/v1/gonum v0.16.0
	google.golang.org/protobuf v1.36.9
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/hashicorp/go-bexpr v0.1.14 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/mitchellh/mapstructure v1.4.1 // indirect
	github.com/mitchellh/pointerstructure v1.2.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/puzpuzpuz/xsync/v3 v3.5.1 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/tmthrgd/go-hex v0.0.0-20190904060850-447a3041c3bc // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	golang.org/x/crypto v0.42.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	mellium.im/sasl v0.3.2 // indirect
)

// required until grid is published
replace github.com/terraconstructs/grid/api v0.0.0 => ../../api
