module github.com/terraconstructs/grid/tests

go 1.24.0

toolchain go1.24.4

// required until grid is published
replace (
	github.com/terraconstructs/grid/pkg/api v0.0.0 => ../pkg/api
	github.com/terraconstructs/grid/pkg/sdk v0.0.0 => ../pkg/sdk
)

require (
	connectrpc.com/connect v1.19.0
	github.com/gofrs/uuid v4.4.0+incompatible
	github.com/google/uuid v1.6.0
	github.com/hashicorp/terraform-exec v0.24.0
	github.com/lib/pq v1.10.9
	github.com/stretchr/testify v1.11.1
	github.com/terraconstructs/grid/pkg/sdk v0.0.0
	gopkg.in/square/go-jose.v2 v2.6.0
)

require (
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-jose/go-jose/v4 v4.1.3 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gorilla/securecookie v1.1.2 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/terraform-json v0.27.1 // indirect
	github.com/muhlemmer/gu v0.3.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/terraconstructs/grid/pkg/api v0.0.0 // indirect
	github.com/zclconf/go-cty v1.16.4 // indirect
	github.com/zitadel/logging v0.6.2 // indirect
	github.com/zitadel/oidc/v3 v3.45.0 // indirect
	github.com/zitadel/schema v1.3.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel v1.38.0 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	golang.org/x/crypto v0.42.0 // indirect
	golang.org/x/oauth2 v0.31.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
