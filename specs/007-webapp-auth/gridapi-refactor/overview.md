# Grid API Refactoring: Overview

**Created**: 2025-11-12
**Status**: Approved - Ready to implement
**Epic**: Grid API race condition fix and proper layering

## Problem Statement

The Grid API authentication middleware suffers from a **critical race condition** that causes intermittent 403 authorization failures for webapp users authenticating with external IdP (Keycloak).

### Symptoms

- **Intermittent 403 errors**: Valid authenticated users randomly rejected (30% failure rate under load)
- **Performance degradation**: 150ms average latency vs 50ms expected
- **Database hotspot**: 600 writes/second to `casbin_rules` table under 100 concurrent users
- **Lock contention**: Global `casbinMutex` becomes bottleneck at scale

### Root Causes

1. **Casbin as per-request mutable state**: Using Casbin enforcer (designed for static policies) as temporary storage for dynamic group→role mappings
2. **AutoSave write amplification**: Every `AddGroupingPolicy()` immediately writes to database (9 queries per authenticated request)
3. **Temporal separation**: Principal resolution (step 3) and authorization (step 6) have no isolation - another request can modify Casbin state between these steps
4. **Layering violations**: 26 violations across handlers and middleware accessing repositories directly
5. **No service layer**: Authentication logic scattered across 5 middleware files with duplicated code

## Solution Strategy

Adopt **attribute-based authorization** with **immutable caching**:

1. **Resolve roles ONCE** at authentication time using immutable cache
2. **Store in request context** as part of Principal struct (immutable)
3. **Authorize using Principal.Roles** against Casbin policies (read-only, no mutation)
4. **Use atomic.Value** for lock-free group→role cache reads (copy-on-write pattern)
5. **Introduce IAM service layer** to centralize all identity/access management logic
6. **Proper layering**: Handlers → Services → Repositories (strict enforcement)

### Key Architectural Principles

- ✅ **Immutable caches**: Use `atomic.Value` with copy-on-write snapshots
- ✅ **Authenticator interface**: Multiple implementations (JWT, Session) producing unified `Principal`
- ✅ **Read-only request path**: No repository writes during request handling
- ✅ **Out-of-band cache refresh**: Background goroutine or admin API for reload
- ✅ **Keep RBAC**: Maintain dynamic group→role mappings and customer-defined policies

## Success Criteria

### Core Fix (Must Have)
- [ ] **Zero race conditions**: `go test -race` passes on all tests
- [ ] **All integration tests passing**: 32/32 tests pass under load (1000+ requests)
- [ ] **No spurious 403s**: Valid principals never rejected
- [ ] **Reduced DB load**: 2-3 queries per request (down from 9)
- [ ] **Improved latency**: < 50ms request time (down from 150ms)
- [ ] **Zero Casbin writes**: No policy mutations during request handling

### Proper Layering (Must Have)
- [ ] **Zero repository imports in handlers**: All handlers use services only
- [ ] **Zero repository imports in middleware**: Middleware uses IAM service
- [ ] **All services in services/**: Proper directory structure
- [ ] **IAM logic centralized**: Single service for all auth concerns

### Production Ready (Should Have)
- [ ] **Background cache refresh**: Automatic reload every N minutes
- [ ] **Admin API**: Manual cache refresh endpoint
- [ ] **Documentation updated**: Architecture diagrams, layering rules
- [ ] **Code review approved**: All phases reviewed and signed off

## Phases Overview

| Phase | Name | Priority | Effort | Risk |
|-------|------|----------|--------|------|
| 1 | Services Layer Foundation | P0 | 6-8h | Low |
| 2 | Immutable Group→Role Cache | P0 | 4-6h | Low |
| 3 | Authenticator Pattern | P0 | 6-8h | Medium |
| 4 | Authorization Refactor | P0 | 4-6h | Low |
| 5 | Move Services to services/ | P1 | 2-4h | Low |
| 6 | Handler Refactor | P1 | 4-6h | Medium |
| 7 | Cache Refresh & Admin API | P2 | 4-6h | Low |
| 8 | Testing & Validation | P0 | 8-10h | Low |
| 9 | Documentation & Cleanup | P2 | 2-4h | None |

**Total Effort**: 40-50 hours (2-3 weeks)

## Impact Analysis

### Performance Improvements

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| DB writes per request | 9 | 0 | 100% reduction |
| Request latency (avg) | 150ms | <50ms | 67% faster |
| DB writes/sec (100 users) | 600/s | 0/s | Eliminated |
| Lock contention | High (mutex) | None (lock-free) | Eliminated |
| Race conditions | Frequent | None | Fixed |

### Code Quality Improvements

| Aspect | Before | After | Improvement |
|--------|--------|-------|-------------|
| Layering violations | 26 | 0 | 100% fixed |
| Authentication logic | 5 files | 1 service | Centralized |
| Lines of middleware code | 1521 LOC | ~500 LOC | 67% reduction |
| Test coverage (IAM) | None | 90%+ | Testable |

## References

- **Detailed Analysis**: [architecture-analysis.md](architecture-analysis.md)
- **Race Condition Evidence**: [GRID-80AD-FIX-SUMMARY.md](../GRID-80AD-FIX-SUMMARY.md)
- **Layering Rules**: [cmd/gridapi/layering.md](../../../cmd/gridapi/layering.md)
- **Timeline & Risks**: [timeline-and-risks.md](timeline-and-risks.md)

## Phase Documents

1. [Phase 1: Services Foundation](phase-1-services-foundation.md)
2. [Phase 2: Immutable Cache](phase-2-immutable-cache.md)
3. [Phase 3: Authenticator Pattern](phase-3-authenticator-pattern.md)
4. [Phase 4: Authorization Refactor](phase-4-authorization-refactor.md)
5. [Phase 5: Move Services](phase-5-move-services.md)
6. [Phase 6: Handler Refactor](phase-6-handler-refactor.md)
7. [Phase 7: Cache Refresh](phase-7-cache-refresh.md)
8. [Phase 8: Testing](phase-8-testing.md)
9. [Phase 9: Documentation](phase-9-documentation.md)
