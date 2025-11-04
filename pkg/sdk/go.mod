module github.com/terraconstructs/grid/pkg/sdk

go 1.24.0

toolchain go1.24.4

require (
	connectrpc.com/connect v1.19.0
	github.com/google/uuid v1.6.0
	github.com/terraconstructs/grid/api v0.0.0
	github.com/zitadel/oidc/v3 v3.45.0
	golang.org/x/oauth2 v0.31.0
	google.golang.org/protobuf v1.36.9
)

require (
	github.com/go-jose/go-jose/v4 v4.1.3 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gorilla/securecookie v1.1.2 // indirect
	github.com/muhlemmer/gu v0.3.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/zitadel/logging v0.6.2 // indirect
	github.com/zitadel/schema v1.3.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel v1.38.0 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	golang.org/x/net v0.44.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)

// required until grid is published
replace github.com/terraconstructs/grid/api v0.0.0 => ../../api
