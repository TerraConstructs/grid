module github.com/terraconstructs/grid/pkg/sdk

go 1.24.0

toolchain go1.24.4

require (
	connectrpc.com/connect v1.19.0
	github.com/google/uuid v1.6.0
	github.com/terraconstructs/grid/api v0.0.0
	google.golang.org/protobuf v1.36.9
)

// required until grid is published
replace github.com/terraconstructs/grid/api v0.0.0 => ../../api
