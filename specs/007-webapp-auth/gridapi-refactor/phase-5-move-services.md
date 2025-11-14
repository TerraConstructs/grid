# Phase 5: Move Services to services/

**Priority**: P1
**Effort**: 2-4 hours
**Risk**: Low
**Dependencies**: Phase 4 complete

## Objectives

- Proper directory structure
- All services in `internal/services/`
- Clean import paths

## Tasks

### Task 5.1: Move Service Packages

```bash
git mv cmd/gridapi/internal/state cmd/gridapi/internal/services/state
git mv cmd/gridapi/internal/dependency cmd/gridapi/internal/services/dependency
git mv cmd/gridapi/internal/graph cmd/gridapi/internal/services/graph
git mv cmd/gridapi/internal/tfstate cmd/gridapi/internal/services/tfstate
```

### Task 5.2: Update Import Paths

Use sed or IDE refactoring to update all imports:

```bash
find cmd/gridapi -name "*.go" -exec sed -i '' 's|internal/state|internal/services/state|g' {} +
find cmd/gridapi -name "*.go" -exec sed -i '' 's|internal/dependency|internal/services/dependency|g' {} +
find cmd/gridapi -name "*.go" -exec sed -i '' 's|internal/graph|internal/services/graph|g' {} +
find cmd/gridapi -name "*.go" -exec sed -i '' 's|internal/tfstate|internal/services/tfstate|g' {} +
```

### Task 5.3: Verify Compilation

```bash
cd cmd/gridapi
go build ./...
go test ./...
```

## Deliverables

- [ ] All services moved to `internal/services/`
- [ ] All imports updated
- [ ] Project compiles successfully
- [ ] All tests pass (32/32 integration tests)

## Related Documents

- **Previous**: [phase-4-authorization-refactor.md](phase-4-authorization-refactor.md)
- **Next**: [phase-6-handler-refactor.md](phase-6-handler-refactor.md)
