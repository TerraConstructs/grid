# Critical Findings from Terraform Source Code Review

**Date**: 2025-09-30
**Reviewed**: `github.com/opentofu/opentofu/internal/backend/remote-state/http/backend.go`

## Key Findings from Unit Test Review

**Source**: `github.com/opentofu/opentofu/internal/backend/remote-state/http/backend_test.go`

### Test: TestHTTPClientFactory

**Default behavior validation**:
```go
// When only address is specified:
client.UpdateMethod = "POST"        // Default update method
client.LockURL = nil                // No lock endpoint
client.LockMethod = "LOCK"          // Default method (even though URL is nil)
client.UnlockURL = nil              // No unlock endpoint
client.UnlockMethod = "UNLOCK"      // Default method (even though URL is nil)
```

**Custom configuration validation**:
```go
"update_method":  "BLAH"   // Accepts ANY string
"lock_method":    "BLIP"   // Accepts ANY string
"unlock_method":  "BLOOP"  // Accepts ANY string
"lock_address":   "http://127.0.0.1:8888/bar"   // Different from state address!
"unlock_address": "http://127.0.0.1:8888/baz"   // Different from lock address!
```

### Critical Insights

1. **Locking is OPTIONAL**:
   - If `lock_address` not specified → no locking (client.LockURL = nil)
   - If `unlock_address` not specified → no unlocking (client.UnlockURL = nil)
   - Methods still have defaults even when URLs are nil

2. **Addresses are INDEPENDENT**:
   - State address: `http://server/state`
   - Lock address: `http://server/lock` (different path!)
   - Unlock address: `http://server/unlock` (different path!)
   - Can even be different hosts (though unusual)

3. **Methods are FULLY CONFIGURABLE**:
   - Not limited to standard HTTP verbs
   - Test uses "BLAH", "BLIP", "BLOOP" to prove ANY string accepted
   - Server must be flexible with method routing

4. **Environment Variable Support**:
   - All settings have `TF_HTTP_*` environment variable equivalents
   - `TF_HTTP_LOCK_METHOD`, `TF_HTTP_UNLOCK_METHOD`, etc.

## Caution: Custom HTTP Methods

### Discovery

Terraform HTTP Backend uses **custom HTTP methods** that are not standard REST verbs by default:

```go
// From Terraform/OpenTofu source
"lock_method": &schema.Schema{
    Type:        schema.TypeString,
    Optional:    true,
    DefaultFunc: schema.EnvDefaultFunc("TF_HTTP_LOCK_METHOD", "LOCK"),
    Description: "The HTTP method to use when locking",
},
"unlock_method": &schema.Schema{
    Type:        schema.TypeString,
    Optional:    true,
    DefaultFunc: schema.EnvDefaultFunc("TF_HTTP_UNLOCK_METHOD", "UNLOCK"),
    Description: "The HTTP method to use when unlocking",
},
```

### Impact

**Default Terraform behavior**:
- `LOCK /tfstate/{guid}/lock` (not PUT)
- `UNLOCK /tfstate/{guid}/unlock` (not PUT)

**Our initial OpenAPI spec**: Used PUT (incorrect for default config)

### Resolution

✅ **Updated artifacts**:
1. `contracts/terraform-backend-rest.yaml`:
   - Added header warning about custom methods
   - Documented LOCK/UNLOCK as actual methods
   - Noted OpenAPI 3.0 limitation (cannot document custom methods)
   - Kept PUT in spec for documentation purposes

2. `research.md` Section 1:
   - Added "Critical Discovery from Source Code Review"
   - Documented method configurability
   - Added Chi router implementation pattern
   - Explained OpenAPI limitation

3. `contracts/README.md`:
   - Added "CRITICAL - Custom HTTP Methods" section
   - Listed actual methods vs OpenAPI documentation
   - Noted implementation requirement

### Implications for Grid Implementation

**1. Locking is Optional**
- ✅ Our implementation: Always provides lock/unlock addresses in backend config
- ⚠️ Users can configure Terraform without locking by omitting lock_address
- Decision: Still implement locking support (required by spec FR-014), users can ignore

**2. Address Independence Not Supported** ⚠️
- ❌ Terraform allows: lock_address ≠ state address
- ✅ Grid design: lock_address = `{state_address}/lock`
- Justification: Simplifies implementation, lock belongs to state
- Trade-off: Users cannot point lock to different service
- Constitutional alignment: Simplicity (Principle VII)

**3. Method Flexibility Required**
- ✅ Must support LOCK/UNLOCK (defaults)
- ✅ Should support common alternatives (PUT, POST, DELETE)
- ❌ Don't need to support arbitrary methods like "BLAH", "BLIP"
- Decision: Whitelist approach (LOCK, UNLOCK, PUT, POST, DELETE, PATCH)

**4. Environment Variable Support**
- ℹ️ Client-side only (Terraform reads env vars)
- ✅ No server implementation needed
- Users can set `TF_HTTP_LOCK_METHOD=PUT` to override config

### Implementation Requirements

**Chi Router Pattern** (revised based on tests):
```go
// Whitelist of supported methods for flexibility
supportedMethods := []string{"LOCK", "PUT", "POST"}
for _, method := range supportedMethods {
    r.Method(method, "/tfstate/{guid}/lock", lockHandler)
}

supportedMethods = []string{"UNLOCK", "PUT", "DELETE", "POST"}
for _, method := range supportedMethods {
    r.Method(method, "/tfstate/{guid}/unlock", unlockHandler)
}

// Or use a catch-all approach in handler:
r.HandleFunc("/tfstate/{guid}/lock", func(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case "LOCK", "PUT", "POST":
        lockHandler(w, r)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
})
```

Chi router supports custom methods via `r.Method(method, path, handler)`.

### Testing Impact

**Unit tests MUST validate** (mirroring Terraform's test structure):
1. **Default behavior**:
   - Only `address` specified in backend config
   - Verify POST for updates
   - Verify LOCK/UNLOCK methods work even when lock_address not set

2. **Custom configuration**:
   - Override `update_method` to non-standard value
   - Override `lock_method` and `unlock_method`
   - Verify custom headers pass through
   - Verify HTTP Basic Auth (username/password) when implemented

3. **Method whitelist**:
   - Accept: LOCK, UNLOCK, PUT, POST, DELETE, PATCH
   - Reject: Arbitrary strings ("BLAH", "BLIP", "BLOOP")
   - Return 405 Method Not Allowed for unsupported methods

**Integration tests MUST validate**:
1. Real Terraform CLI with default config (LOCK/UNLOCK methods)
2. Configured alternatives: `lock_method="PUT"`, `unlock_method="DELETE"`
3. Optional locking: Backend config without lock_address/unlock_address
4. Environment variables: `TF_HTTP_LOCK_METHOD=PUT`

**Contract test matrix**:
```go
// Test various method combinations
testCases := []struct{
    updateMethod, lockMethod, unlockMethod string
    shouldAccept bool
}{
    {"POST", "LOCK", "UNLOCK", true},      // Default Terraform
    {"PUT", "PUT", "DELETE", true},        // Common alternative
    {"PATCH", "POST", "POST", true},       // Valid but unusual
    {"BLAH", "BLIP", "BLOOP", false},      // Arbitrary (reject with 405)
}
```

## Other Schema Fields Reviewed

### Configurable (User Can Override)

1. **update_method**: Default "POST"
   - User can set to PUT, PATCH, etc.
   - Server should support POST as minimum

2. **Retry Configuration** (client-side, not server concern):
   - `retry_max`: Default 2
   - `retry_wait_min`: Default 1 second
   - `retry_wait_max`: Default 30 seconds

3. **Custom Headers**:
   - `headers`: Map of custom headers
   - Reserved: "Content-Type", "Content-MD5"
   - "Authorization" conflicts with username/password

### Authentication (Deferred to Future Version)

1. **HTTP Basic Auth**:
   - `username` / `password`
   - Conflicts with "Authorization" header

2. **TLS/mTLS**:
   - `skip_cert_verification`: For development
   - `client_ca_certificate_pem`: CA cert for server verification
   - `client_certificate_pem` + `client_private_key_pem`: mTLS

3. **Custom Headers**:
   - Can include "Authorization" if username not set

**Decision**: No authentication in initial version (per specification)

## Recommendations

### Immediate (Required for v0.1.0)

1. ✅ Support LOCK/UNLOCK custom HTTP methods
2. ✅ Document in contracts and research
3. ✅ Update integration tests to use LOCK/UNLOCK
4. ⏳ Implement Chi router with custom methods

### Future Enhancements

1. **Method Configuration Support**:
   - Allow users to configure which methods to accept
   - Environment variables or config file
   - Default: LOCK/UNLOCK/PUT (liberal acceptance)

2. **Authentication**:
   - HTTP Basic Auth (username/password)
   - Custom headers (Authorization bearer tokens)
   - mTLS (client certificates)

3. **Custom Headers**:
   - Pass-through from Terraform to backend
   - Useful for request tracing, auth tokens

## Constitutional Compliance

✅ **No violations**:
- Custom methods don't break SDK-first (Terraform REST exception is documented)
- Simplicity maintained (custom methods are protocol requirement, not choice)
- Repository pattern unaffected (HTTP layer only)

## Summary of Changes Made

### Documentation Updates

1. **`contracts/terraform-backend-rest.yaml`** ✅
   - Added warning about LOCK/UNLOCK custom methods in description
   - Documented OpenAPI 3.0 limitation
   - Clarified PUT shown for documentation only
   - Added authentication notes (future)

2. **`research.md` Section 1** ✅
   - Added method whitelist implementation pattern
   - Documented locking as optional feature
   - Explained address independence trade-off (Grid simplifies)
   - Added Chi router code examples with whitelisted methods

3. **`contracts/README.md`** ✅
   - Added "CRITICAL - Custom HTTP Methods" warning section
   - Listed actual methods vs OpenAPI docs
   - Documented implementation requirements

4. **`CRITICAL-FINDINGS.md`** ✅ (This file)
   - Comprehensive source code and unit test analysis
   - Implementation implications and trade-offs
   - Testing requirements with method matrix
   - Future enhancement roadmap

5. **`quickstart.md`** ✅
   - Added "Advanced: Configuring Alternative HTTP Methods" section
   - Added "Advanced: Disabling Locking" section
   - Documented supported methods and environment variables

### Design Decisions Documented

| Aspect | Terraform Capability | Grid Implementation | Rationale |
|--------|---------------------|---------------------|-----------|
| **Lock/Unlock Methods** | LOCK/UNLOCK (custom) | ✅ Support + alternatives | Required for default config |
| **Method Configuration** | ANY string accepted | ⚠️ Whitelist common methods | Simplicity (Constitution VII) |
| **Lock Address Independence** | Can differ from state address | ❌ Derived from state address | Simplicity - lock belongs to state |
| **Optional Locking** | lock_address can be nil | ✅ Always provide, user can ignore | Spec requires locking (FR-014) |
| **Method Whitelist** | N/A (client-side) | POST, PUT, PATCH, LOCK, UNLOCK, DELETE | Balance flexibility + security |

### Risk Assessment

**Low Risk (Addressed)**:
- ✅ Custom HTTP method support via Chi router
- ✅ Method whitelisting prevents arbitrary method abuse
- ✅ OpenAPI documentation clarified (limitation noted)

**Medium Risk (Accepted Trade-offs)**:
- ⚠️ No support for independent lock addresses (simplicity over flexibility)
- ⚠️ Limited method whitelist (common methods only, not arbitrary strings)
- Users can work around by configuring supported alternative methods

**No Risk**:
- ❌ No authentication concerns (initial version explicitly unauthenticated)
- ❌ No performance concerns (method routing is O(1))

## References

- Terraform HTTP Backend Protocol: https://developer.hashicorp.com/terraform/language/settings/backends/http
- OpenTofu Source: `github.com/opentofu/opentofu/internal/backend/remote-state/http/backend.go`
- OpenTofu Unit Tests: `github.com/opentofu/opentofu/internal/backend/remote-state/http/backend_test.go`
- Chi Router Custom Methods: https://github.com/go-chi/chi#routing
- OpenAPI 3.0 Custom Methods Issue: https://github.com/OAI/OpenAPI-Specification/issues/1545