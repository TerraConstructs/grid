module github.com/terraconstructs/grid/cmd/gridctl

go 1.24.0

toolchain go1.24.4

// required until grid is published
replace (
	github.com/terraconstructs/grid/api v0.0.0 => ../../api
	github.com/terraconstructs/grid/pkg/sdk v0.0.0 => ../../pkg/sdk
)

require (
	github.com/google/uuid v1.6.0
	github.com/spf13/cobra v1.10.1
	github.com/terraconstructs/grid/pkg/sdk v0.0.0
)

require (
	connectrpc.com/connect v1.19.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/terraconstructs/grid/api v0.0.0 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
)
