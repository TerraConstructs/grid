package auth

// Phase 4 Note: ApplyDynamicGroupings, clearUserGroupings, GetEffectiveRoles,
// and ResolveGroupRoles were removed.
//
// These functions relied on Casbin dynamic grouping policies which caused:
//   - Race conditions (concurrent AddGroupingPolicy calls)
//   - Write amplification (9 DB queries + Casbin mutation per request)
//   - Global mutex bottleneck (casbinMutex)
//
// The new architecture (Phases 1-4) uses:
//   - Immutable GroupRoleCache (atomic.Value) for lock-free reads
//   - Pre-resolved roles in Principal.Roles at authentication time
//   - Read-only Casbin authorization (no state mutation)
//   - IAM Service handles all role resolution via ResolveRoles method
