
# Forked Adapter

Internal fork of the [Bun Casbin adapter](https://github.com/msales/casbin-bun-adapter/blob/v1.0.7/adapter.go) so the CasbinRule model no longer hard-codes the `casbin.` schema, keeping the original API but allowing the same table name to work in Postgres (public search path) and SQLite.
