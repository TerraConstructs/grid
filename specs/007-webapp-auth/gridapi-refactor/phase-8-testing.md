# Phase 8: Testing & Validation

**Priority**: P0
**Effort**: 8-10 hours
**Risk**: Low
**Dependencies**: Phase 1-7 complete

## Objectives

- Verify race condition eliminated
- All integration tests passing
- Load testing under high concurrency
- Performance metrics validated

## Tasks

### Task 8.1: Run Mode 1 Integration Tests

**Command**:
```bash
make test-integration-mode1
```

**Expected Results**:
- ✅ 18/18 tests pass
- ✅ No race conditions (`go test -race`)
- ✅ No 403 errors for valid principals
- ✅ Test duration < 60 seconds

**Tests**:
1. TestMode1_KeycloakHealth
2. TestMode1_ExternalTokenValidation
3. TestMode1_ServiceAccountAuth
4. TestMode1_UserGroupAuthorization
5. TestMode1_SSO_WebFlow
6. TestMode1_DeviceFlow
7. TestMode1_TokenExpiry
8. TestMode1_InvalidTokenRejection
9. TestMode1_SSO_UserAuth
10. TestMode1_GroupRoleMapping
11. TestMode1_GroupRoleMapping_UnionSemantics
12. TestMode1_DependencyAuthorization_HappyPath
13. TestMode1_DependencyAuthorization_CrossScopeSourceDenial
14. TestMode1_DependencyAuthorization_CrossScopeDestinationDenial
15. TestMode1_DependencyAuthorization_ListAndDelete
16. TestMode1_StateOutputAuthorization_HappyPath
17. TestMode1_StateOutputAuthorization_CrossScopeDenial
18. TestMode1_StateOutputAuthorization_WriteViaTerraform

### Task 8.2: Run Mode 2 Integration Tests

**Command**:
```bash
make test-integration-mode2
```

**Expected Results**:
- ✅ 14/14 tests pass
- ✅ No race conditions
- ✅ All web auth flows working

**Tests**:
1. TestMode2_SigningKeyGeneration
2. TestMode2_ServiceAccountBootstrap
3. TestMode2_ServiceAccountAuthentication
4. TestMode2_AuthenticatedAPICall
5. TestMode2_JWTRevocation
6. TestMode2_WebAuth_LoginSuccess
7. TestMode2_WebAuth_LoginCookieAttributes
8. TestMode2_WebAuth_LoginInvalidCredentials
9. TestMode2_WebAuth_LoginDisabledAccount
10. TestMode2_WebAuth_WhoamiSuccess
11. TestMode2_WebAuth_WhoamiUnauthenticated
12. TestMode2_WebAuth_AuthConfig
13. TestMode2_WebAuth_LogoutSuccess
14. TestMode2_WebAuth_FullFlow

### Task 8.3: Run Load Tests

**Scenario**: 100 concurrent webapp users, external IdP, 2 groups each

**Test Script** (`tests/integration/load_test.go`):
```go
func TestLoadTest_MultiAuth(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping load test in short mode")
    }

    // Setup: Create 100 test users with sessions
    users := setupTestUsers(t, 100)

    var wg sync.WaitGroup
    successCount := atomic.Int64{}
    errorCount := atomic.Int64{}

    // Simulate 100 concurrent users, each making 10 requests
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(userIdx int) {
            defer wg.Done()

            client := createAuthenticatedClient(users[userIdx])

            for j := 0; j < 10; j++ {
                resp, err := client.ListStates(context.Background())
                if err != nil {
                    errorCount.Add(1)
                    t.Errorf("User %d request %d failed: %v", userIdx, j, err)
                } else {
                    successCount.Add(1)
                }
            }
        }(i)
    }

    wg.Wait()

    t.Logf("Load test complete: %d successes, %d errors", successCount.Load(), errorCount.Load())
    require.Equal(t, int64(1000), successCount.Load(), "All requests should succeed")
    require.Equal(t, int64(0), errorCount.Load(), "No requests should fail")
}
```

**Run**:
```bash
go test -v -run TestLoadTest_MultiAuth ./tests/integration/
```

**Expected**:
- ✅ 1000/1000 requests succeed
- ✅ 0 failures
- ✅ Average latency < 50ms
- ✅ No race conditions

### Task 8.4: Validate Performance Metrics

**Metrics to Collect**:

| Metric | Before | After | Target | Status |
|--------|--------|-------|--------|--------|
| DB writes per request | 9 | ? | 0 | ⬜ |
| Request latency (avg) | 150ms | ? | <50ms | ⬜ |
| DB writes/sec (100 users) | 600/s | ? | 0/s | ⬜ |
| Lock contention | High | ? | None | ⬜ |
| Race conditions | Frequent | ? | None | ⬜ |
| Cache hit rate | N/A | ? | >99% | ⬜ |

**Measurement Tools**:
- DB writes: `SELECT count(*) FROM casbin_rules;` before and after test run
- Latency: Go benchmarks or integration test timing
- Lock contention: `go test -race -cpuprofile=cpu.prof`
- Cache hit rate: Add metrics to `GroupRoleCache.Get()`

## Deliverables

- [ ] Mode 1 tests: 18/18 passing
- [ ] Mode 2 tests: 14/14 passing
- [ ] Load tests pass (1000/1000 requests succeed)
- [ ] Performance metrics meet targets
- [ ] No race conditions detected
- [ ] Test report written

## Test Report Template

**File**: `specs/007-webapp-auth/gridapi-refactor/test-report.md`

```markdown
# Refactoring Test Report

**Date**: YYYY-MM-DD
**Tester**: [Name]

## Integration Tests

### Mode 1 (External IdP)
- Status: ✅ PASS / ❌ FAIL
- Tests passed: X/18
- Duration: Xs
- Race detector: ✅ Clean / ❌ Issues found
- Notes: [Any issues encountered]

### Mode 2 (Internal IdP)
- Status: ✅ PASS / ❌ FAIL
- Tests passed: X/14
- Duration: Xs
- Race detector: ✅ Clean / ❌ Issues found
- Notes: [Any issues encountered]

## Load Tests

- Concurrent users: 100
- Requests per user: 10
- Total requests: 1000
- Success rate: X%
- Average latency: Xms
- Max latency: Xms
- Race conditions: ✅ None / ❌ Detected

## Performance Metrics

[Table with before/after metrics]

## Conclusion

[Pass/Fail, next steps]
```

## Related Documents

- **Previous**: [phase-7-cache-refresh.md](phase-7-cache-refresh.md)
- **Next**: [phase-9-documentation.md](phase-9-documentation.md)


tips:

To capture and summarize the output of the Mode 2 integration tests, you can use the following command:

```
make test-integration-mode2 2>&1 | tee /tmp/mode2-test-output.txt | grep -E "(^=== RUN|^--- PASS|^--- FAIL|PASS:|FAIL:)" | head -50
```