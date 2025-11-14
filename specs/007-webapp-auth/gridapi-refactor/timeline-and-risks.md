# Timeline and Risk Management

**Last Updated**: 2025-11-12

## Timeline

### Sprint 1 (Week 1): Core Architecture

**Days 1-2**: Phase 1 (Services Foundation)
- Create `internal/services/iam/` directory
- Define Authenticator interface
- Define Principal struct
- Define IAM Service interface
- **Deliverable**: Interfaces compile, documented

**Days 3-4**: Phase 2 (Immutable Cache)
- Implement GroupRoleCache with atomic.Value
- Write unit tests (including concurrency test)
- Integrate into IAM service
- **Deliverable**: Cache working, tests pass with race detector

**Day 5**: Phase 3 (Authenticator Pattern)
- Implement JWTAuthenticator
- Implement SessionAuthenticator
- Implement MultiAuth middleware
- **Deliverable**: Both auth paths produce identical Principal

### Sprint 2 (Week 2): Refactoring & Testing

**Days 6-7**: Phase 4 (Authorization Refactor)
- Implement read-only Casbin authorization
- Refactor authz middleware to use principal.Roles
- Remove all Casbin mutation code
- Delete casbinMutex
- **Deliverable**: Zero Casbin writes during requests

**Day 8**: Phase 5 (Move Services)
- Move state, dependency, graph, tfstate to services/
- Update imports
- **Deliverable**: Proper directory structure

**Day 9**: Phase 6 (Handler Refactor)
- Refactor handlers to use IAM service
- Remove repository dependencies
- Delete auth_helpers.go
- **Deliverable**: Zero layering violations

**Day 10**: Phase 8 (Testing)
- Run Mode 1 integration tests
- Run Mode 2 integration tests
- Run load tests
- Validate performance metrics
- **Deliverable**: All tests pass, metrics validated

### Sprint 3 (Week 3): Polish

**Day 11**: Phase 7 (Background Refresh)
- Background cache refresh goroutine
- Admin API for manual refresh
- **Deliverable**: Operational cache management

**Day 12**: Phase 9 (Documentation)
- Update layering.md
- Clean up old files
- Write SUMMARY.md
- Code review
- **Deliverable**: Documentation complete, code reviewed

## Risk Management

### Risk Matrix

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Integration tests fail after refactor | Medium | High | Run tests after each phase; rollback if failures |
| Performance regression | Low | Medium | Benchmark before/after; immutable cache faster than mutex |
| Circular dependency (IAM ↔ Authenticators) | Low | Medium | Use factory pattern: IAM service creates authenticators |
| Cache staleness (group→role changes) | Low | Low | Background refresh (5min), auto-refresh on admin ops |
| Backward compatibility break | Low | High | Keep existing interfaces during migration |
| Race conditions in cache | Low | High | Extensive concurrency testing, race detector |
| Missing edge cases | Medium | Medium | Comprehensive test coverage (>90%) |

### Detailed Risk Mitigation

#### Risk 1: Integration Tests Fail

**Mitigation**:
- Run full test suite after EACH phase
- If tests fail, rollback phase and debug
- Use feature flags if partial rollout needed
- Keep old middleware code until new code proven

**Rollback Plan**:
```bash
# If Phase N fails
git checkout HEAD~1  # Revert last commit
git push --force-with-lease origin 007-webapp-auth-refactor

# Re-run tests
make test-integration-mode1
make test-integration-mode2
```

#### Risk 2: Performance Regression

**Mitigation**:
- Benchmark BEFORE starting refactoring (baseline)
- Benchmark AFTER each major phase
- Compare: request latency, DB queries, throughput

**Benchmarking**:
```bash
# Before
go test -bench=. -benchmem ./tests/integration/ > before.txt

# After Phase 2
go test -bench=. -benchmem ./tests/integration/ > after-phase2.txt

# Compare
benchstat before.txt after-phase2.txt
```

**Expected Improvements**:
- Latency: 50-70% reduction
- DB queries: 70% reduction
- Throughput: 2-3x increase

#### Risk 3: Circular Dependency

**Problem**: IAM service needs authenticators, authenticators need IAM service (for role resolution)

**Solution**: Factory pattern
```go
// IAM service creates authenticators, passes itself as dependency
func NewIAMService(deps) (Service, error) {
    svc := &iamService{...}

    // Create authenticators after service exists
    svc.authenticators = []Authenticator{
        NewSessionAuthenticator(deps.Sessions, deps.Users, svc),
        NewJWTAuthenticator(deps.Verifier, deps.Config, svc),
    }

    return svc, nil
}
```

#### Risk 4: Cache Staleness

**Problem**: Group→role mappings change, cache not updated, users get old roles

**Mitigation**:
1. **Background refresh**: Every 5 minutes (configurable)
2. **Automatic refresh**: After AssignGroupRole/RemoveGroupRole
3. **Manual refresh**: Admin API endpoint
4. **Monitoring**: Log cache version on each refresh

**Acceptable Staleness**: Up to 5 minutes is OK for group→role changes (not security-critical window)

#### Risk 5: Backward Compatibility

**Problem**: Existing middleware expects specific context structure

**Mitigation**:
- Keep `auth.AuthenticatedPrincipal` struct unchanged
- Provide conversion function: `convertToLegacy(iam.Principal)`
- Phase migration: new code produces old structs
- Later: update all consumers to use new struct

#### Risk 6: Race Conditions in Cache

**Mitigation**:
- Use `atomic.Value` correctly (documented pattern)
- Extensive concurrency testing (1000 readers, 1 writer)
- Run with race detector: `go test -race`
- Code review focused on concurrent access

**Test Coverage**:
```go
// Test: 1000 concurrent readers, 10 refreshes
// If race detector finds issues, they'll be caught here
```

## Contingency Plans

### Plan A: Full Rollout (Target)

Complete all 9 phases in 2-3 weeks, deploy to production.

### Plan B: Phased Rollout

If issues found:
1. Complete Phases 1-4 (core fix)
2. Deploy core fix only
3. Monitor for 1 week
4. Continue with Phases 5-9 if stable

### Plan C: Feature Flag

If high risk:
1. Add feature flag: `ENABLE_IAM_SERVICE=true/false`
2. Deploy with flag OFF
3. Enable for small % of traffic
4. Gradually increase to 100%

```go
if config.EnableIAMService {
    // Use new MultiAuth middleware
} else {
    // Use old middleware
}
```

### Plan D: Rollback

If critical issues:
1. Disable feature flag (immediate)
2. Git revert (if flag not available)
3. Debug offline
4. Re-deploy fix

## Success Metrics

### Phase-Level Success (Gate for Next Phase)

After each phase, verify:
- [ ] Code compiles without errors
- [ ] All existing tests pass
- [ ] No new warnings or linter errors
- [ ] Code reviewed by at least one other developer

### Feature-Level Success (Gate for Production)

After all phases complete:
- [ ] All 32 integration tests pass
- [ ] Load tests pass (1000 concurrent requests)
- [ ] Race detector clean
- [ ] Performance metrics meet targets
- [ ] Security review approved
- [ ] Documentation complete

### Production Success (Post-Deployment)

After deploying to production:
- [ ] Zero 403 errors for valid users (monitor for 1 week)
- [ ] Request latency < 50ms (99th percentile)
- [ ] Zero panics or crashes
- [ ] Cache refresh working (logs show periodic refreshes)
- [ ] Memory usage stable (no leaks)

## Monitoring

### Key Metrics to Monitor

1. **Authentication Success Rate**
   - Metric: `gridapi_auth_success_total` / `gridapi_auth_attempts_total`
   - Target: >99.9%
   - Alert: <99%

2. **Authorization Denial Rate**
   - Metric: `gridapi_authz_denied_total` / `gridapi_authz_checks_total`
   - Expected: 1-5% (normal denials)
   - Alert: Sudden spike (possible issue)

3. **Cache Hit Rate**
   - Metric: `gridapi_cache_hits_total` / `gridapi_cache_requests_total`
   - Target: >99%
   - Alert: <95%

4. **Request Latency**
   - Metric: `gridapi_request_duration_seconds` (p50, p95, p99)
   - Target: p50 <20ms, p95 <50ms, p99 <100ms
   - Alert: p99 >200ms

5. **Database Query Count**
   - Metric: `gridapi_db_queries_total` per request
   - Target: 2-3 queries per authenticated request
   - Alert: >5 queries (regression)

## References

- **Overview**: [overview.md](overview.md)
- **Architecture**: [architecture-analysis.md](architecture-analysis.md)
- **Phases**: [phase-*.md](.)
