# Quickstart: State Labels

**Feature Branch**: `005-add-state-dimensions`
**Last Updated**: 2025-10-09 (Scope Reduction Applied)

This quickstart verifies the label lifecycle, policy validation, bexpr filtering, and compliance workflows end-to-end in a local environment. **Note**: Uses JSON column storage with in-memory bexpr filtering; EAV tables and facet projection deferred.

## Prerequisites
- Docker available for Postgres (`make db-up`)
- Go toolchain (`go 1.24.4`)
- Built binaries (`make build` → `./bin/gridapi`, `./bin/gridctl`)

## 1. Start services
```bash
make db-reset && make build && ./bin/gridapi db init && ./bin/gridapi db migrate
./bin/gridapi serve --db-url "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
```

## 2. Seed baseline states with labels
```bash
./bin/gridctl state create demo-staging --label env=staging --label team=core --label region=us-west
./bin/gridctl state create demo-prod --label env=prod --label team=core --label region=us-east
./bin/gridctl state create test-staging --label env=staging --label team=platform --label region=us-west
./bin/gridctl state list
```
- Expect: labels echoed in list output; labels stored in `labels` JSONB column on `states` table

## 3. Set label policy & retrieve enum values
```bash
cat <<'EOF' > /tmp/label-policy.json
{
  "allowed_keys": {"env": {}, "team": {}, "region": {}, "cost_center": {}},
  "allowed_values": {
    "env": ["staging", "prod"],
    "region": ["us-west", "us-east", "eu-west-1"]
  },
  "reserved_prefixes": ["grid.io/"],
  "max_keys": 32,
  "max_value_len": 256
}
EOF

./bin/gridctl policy set --file /tmp/label-policy.json
./bin/gridctl policy get
./bin/gridctl policy enum env
```
- Expect: CLI reports policy version `1`, policy get shows constraints, enum command lists `["staging", "prod"]`

## 4. Query states using bexpr filters
```bash
# Simple equality (CLI sugar)
./bin/gridctl state list --label env=staging
./bin/gridctl state list --label env=staging --label team=core # AND semantics

# Compound bexpr expressions
./bin/gridctl state list --filter 'env in ["staging","prod"] && team == "core"'
./bin/gridctl state list --filter 'region == "us-west" || region == "eu-west-1"'
./bin/gridctl state list --filter 'env == "staging"' --page-size 10
```
- Expect: results returned using in-memory bexpr filtering, pagination supported

## 5. Test label updates and removals
```bash
./bin/gridctl state set demo-staging --label cost_center=eng-001 --label -region
./bin/gridctl state get demo-staging
```
- Expect: `cost_center` added, `region` removed, updated labels displayed in get output

## 6. Validate policy enforcement
```bash
# Try invalid value (should fail)
./bin/gridctl state set demo-prod --label env=qa

# Try reserved namespace (should fail)
./bin/gridctl state create --name test --label grid.io/internal=foo
```
- Expect: Both commands fail with clear validation errors citing policy constraints

## 7. Optional: Compliance reporting
```bash
# Manually force a non-compliant label (bypassing CLI, e.g., via SQL)
# Then run compliance report
./bin/gridctl policy compliance
```
- Expect: CLI lists any states with labels violating current policy (if compliance command is implemented)

## 8. Clean up
```bash
Ctrl+C # stop gridapi
make db-down
```

## Verification Checklist
- [ ] Labels stored in `labels` JSONB column (`SELECT guid, logic_id, labels FROM states`)
- [ ] Optional GIN index on labels column for future SQL push-down (`\d states` in psql)
- [ ] bexpr filtering returns correct states for compound expressions
- [ ] Policy enforcement blocks invalid keys, values, and reserved namespaces
- [ ] Label updates/removals apply atomically
- [ ] Policy versioning tracked in `label_policy` table
- [ ] Pagination works on filtered `state list` (over-fetch + trim pattern)
- [ ] Query performance acceptable (<50ms) for in-memory bexpr filtering at 100-500 state scale
