module github.com/terraconstructs/grid/cmd/gridapi

go 1.24.7

toolchain go1.24.10

require (
	connectrpc.com/connect v1.19.0
	github.com/btcsuite/btcutil v1.0.2
	github.com/casbin/casbin/v2 v2.128.0
	github.com/go-chi/chi/v5 v5.2.3
	github.com/go-chi/cors v1.2.1
	github.com/go-jose/go-jose/v4 v4.1.3
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/google/uuid v1.6.0
	github.com/hashicorp/go-bexpr v0.1.14
	github.com/mitchellh/mapstructure v1.4.1
	github.com/spf13/cobra v1.10.1
	github.com/stretchr/testify v1.11.1
	github.com/terraconstructs/grid/api v0.0.0
	github.com/uptrace/bun v1.2.15
	github.com/uptrace/bun/dialect/pgdialect v1.2.15
	github.com/uptrace/bun/driver/pgdriver v1.2.15
	github.com/xenitab/go-oidc-middleware v0.0.44
	github.com/zitadel/oidc/v3 v3.45.0
	golang.org/x/crypto v0.42.0
	golang.org/x/net v0.44.0
	gonum.org/v1/gonum v0.16.0
	google.golang.org/protobuf v1.36.9
)

require (
	github.com/JLugagne/jsonschema-infer v0.1.2 // indirect
	github.com/benbjohnson/clock v1.3.5 // indirect
	github.com/bmatcuk/doublestar/v4 v4.9.1 // indirect
	github.com/casbin/govaluate v1.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.2.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/gorilla/securecookie v1.1.2 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/lestrrat-go/backoff/v2 v2.0.8 // indirect
	github.com/lestrrat-go/blackmagic v1.0.2 // indirect
	github.com/lestrrat-go/httpcc v1.0.1 // indirect
	github.com/lestrrat-go/iter v1.0.2 // indirect
	github.com/lestrrat-go/jwx v1.2.29 // indirect
	github.com/lestrrat-go/option v1.0.1 // indirect
	github.com/mitchellh/pointerstructure v1.2.1 // indirect
	github.com/muhlemmer/gu v0.3.1 // indirect
	github.com/muhlemmer/httpforwarded v0.1.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/puzpuzpuz/xsync/v3 v3.5.1 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/tmthrgd/go-hex v0.0.0-20190904060850-447a3041c3bc // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/zitadel/logging v0.6.2 // indirect
	github.com/zitadel/schema v1.3.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel v1.38.0 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	go.uber.org/ratelimit v0.3.1 // indirect
	golang.org/x/oauth2 v0.31.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	mellium.im/sasl v0.3.2 // indirect
)

// required until grid is published
replace github.com/terraconstructs/grid/api v0.0.0 => ../../api
