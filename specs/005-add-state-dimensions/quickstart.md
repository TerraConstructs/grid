# Quickstart: State Identity Dimensions

**Feature Branch**: `005-add-state-dimensions`
**Last Updated**: 2025-10-09

This quickstart verifies the tagging, schema validation, filtering, and compliance workflows end-to-end in a local environment. **Note**: Facet projection deferred to future milestone; all queries use indexed EAV joins.

## Prerequisites
- Docker available for Postgres (`make db-up`)
- Go toolchain (`go 1.24.4`)
- Built binaries (`make build` → `./bin/gridapi`, `./bin/gridctl`)

## 1. Start services
```bash
make db-up
./bin/gridapi serve --db-url "postgres://grid:gridpass@localhost:5432/grid?sslmode=disable"
```

## 2. Seed baseline states with tags
```bash
./bin/gridctl state create --name demo-staging --set env=staging --set team=core --set region=us-west
./bin/gridctl state create --name demo-prod --set env=prod --set team=core --set region=us-east
./bin/gridctl state create --name test-staging --set env=staging --set team=platform --set region=us-west
./bin/gridctl state list
```
- Expect: tags echoed in list output; keys/values normalized in `meta_keys`, `meta_values`, `state_metadata` dictionary tables

## 3. Upload initial schema & audit it
```bash
cat <<'EOF' > /tmp/state-schema.json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "env": { "enum": ["staging", "prod"] },
    "team": { "type": "string", "maxLength": 32 },
    "region": { "enum": ["us-west", "us-east"] }
  },
  "additionalProperties": false
}
EOF

./bin/gridctl tags schema set --file /tmp/state-schema.json --reason "Baseline schema"
./bin/gridctl tags audit list
```
- Expect: CLI reports schema version `1`, audit log shows creation entry with diff

## 4. Query states by tag filters
```bash
./bin/gridctl state list --filter env=staging
./bin/gridctl state list --filter env=staging --filter team=core
./bin/gridctl state list --filter region=us-west --page-size 10
```
- Expect: results returned using indexed EAV joins on `state_metadata(key_id, value_id, state_id)`, pagination supported

## 5. Test tag updates and removals
```bash
./bin/gridctl state set demo-staging --set cost-center=eng-001 --set -region
./bin/gridctl state info demo-staging
```
- Expect: `cost-center` added, `region` removed, updated tags displayed in info output

## 6. Validate compliance reporting
```bash
./bin/gridctl state set demo-prod --set env=qa
./bin/gridctl tags compliance report
```
- Expect: CLI lists `demo-prod` as non-compliant (schema only allows `staging`/`prod` for `env`), shows violated rule, state remains accessible

## 7. Audit log management
```bash
./bin/gridctl tags audit list --limit 5
./bin/gridctl tags audit flush --before 2025-10-09 --download ./artifacts/audit-20251008.json
```
- Expect: audit entries listed, then downloaded to JSON file and trimmed from database

## 8. Clean up
```bash
Ctrl+C # stop gridapi
make db-down
```

## Verification Checklist
- [ ] Dictionary tables populated (`SELECT * FROM meta_keys`, `meta_values`, `state_metadata`)
- [ ] Composite index exists on `state_metadata(key_id, value_id, state_id)`
- [ ] Tag filtering returns correct states via indexed EAV queries
- [ ] Compliance report lists non-compliant state with violated schema rule
- [ ] Tag updates/removals apply atomically
- [ ] Audit log captured schema creation and can be flushed
- [ ] Pagination works on filtered `state list`
- [ ] Query performance acceptable (<50ms) for tag filters
