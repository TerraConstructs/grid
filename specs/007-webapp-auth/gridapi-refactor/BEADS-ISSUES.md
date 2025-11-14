# Beads Issues Created for Grid API Refactoring

**Created**: 2025-11-12
**Epic ID**: grid-f21b

This document lists all beads issues created for the Grid API authentication refactoring project.

## Epic

**grid-f21b** - Grid API Authentication Refactoring: Eliminate Race Condition & Proper Layering
- Type: epic
- Priority: P1
- Status: open
- Labels: `spec:007-webapp-auth`, `component:gridapi`, `type:refactoring`, `priority:critical`

## Feature Issues (9 Phases)

### Phase 1: Services Layer Foundation
**grid-5d33** - Phase 1: Services Layer Foundation - IAM Package & Interfaces
- Type: feature
- Priority: P1
- Depends on: grid-f21b (epic)
- Tasks: 4 created (grid-82b8, grid-8673, grid-c734, grid-a741)

#### Tasks
1. **grid-82b8** - Create IAM package structure and documentation
2. **grid-8673** - Define Authenticator interface
3. **grid-c734** - Define Principal struct
4. **grid-a741** - Define IAM Service interface with all methods

### Phase 2: Immutable Group→Role Cache
**grid-3e64** - Phase 2: Immutable Group→Role Cache with atomic.Value
- Type: feature
- Priority: P1
- Depends on: grid-5d33 (Phase 1)
- Tasks: 3 created (grid-e92d, grid-331d, grid-bc01)

#### Tasks
1. **grid-e92d** - Implement GroupRoleCache with atomic.Value
2. **grid-331d** - Write GroupRoleCache unit tests including concurrency test
3. **grid-bc01** - Integrate GroupRoleCache into IAM service

### Phase 3: Authenticator Pattern
**grid-e9da** - Phase 3: Authenticator Pattern Implementation
- Type: feature
- Priority: P1
- Depends on: grid-5d33 (Phase 1)
- Tasks: To be created during implementation

### Phase 4: Authorization Refactor
**grid-314f** - Phase 4: Authorization Refactor - Read-Only Casbin
- Type: feature
- Priority: P1
- Depends on: grid-5d33 (Phase 1)
- Tasks: To be created during implementation

### Phase 5: Move Services
**grid-dcf9** - Phase 5: Move Services to internal/services/
- Type: feature
- Priority: P2
- Depends on: grid-5d33 (Phase 1)
- Tasks: To be created during implementation

### Phase 6: Handler Refactor
**grid-f6b7** - Phase 6: Handler Refactor - Fix Layering Violations
- Type: feature
- Priority: P2
- Depends on: grid-5d33 (Phase 1)
- Tasks: To be created during implementation

### Phase 7: Cache Refresh & Admin API
**grid-4bea** - Phase 7: Cache Refresh & Admin API
- Type: feature
- Priority: P2
- Depends on: grid-5d33 (Phase 1)
- Tasks: To be created during implementation

### Phase 8: Testing & Validation
**grid-c842** - Phase 8: Testing & Validation
- Type: feature
- Priority: P1
- Depends on: grid-5d33 (Phase 1)
- Tasks: To be created during implementation

### Phase 9: Documentation & Cleanup
**grid-8251** - Phase 9: Documentation & Cleanup
- Type: feature
- Priority: P2
- Depends on: grid-5d33 (Phase 1)
- Tasks: To be created during implementation

## Usage

### View all refactoring issues
```bash
bd list --label spec:007-webapp-auth
```

### View epic with dependencies
```bash
bd show grid-f21b
```

### View Phase 1 tasks
```bash
bd list --label phase:1-foundation
```

### View Phase 2 tasks
```bash
bd list --label phase:2-cache
```

### Find ready tasks
```bash
bd ready --label spec:007-webapp-auth
```

### View blocked tasks
```bash
bd blocked --label spec:007-webapp-auth
```

## Task Creation Guide

As you implement each phase, create additional tasks as needed. Example:

```bash
# Create a task for Phase 3
bd create --title "Implement JWTAuthenticator" \
  --description "Implement JWT bearer token authentication..." \
  --type task \
  --priority 1 \
  --deps grid-e9da \
  --labels "spec:007-webapp-auth,component:gridapi,phase:3-authenticator"
```

## Documentation References

- **Overview**: [overview.md](overview.md)
- **Architecture Analysis**: [architecture-analysis.md](architecture-analysis.md)
- **Phase Details**: [phase-*.md](.)
- **Timeline & Risks**: [timeline-and-risks.md](timeline-and-risks.md)

## Summary

**Created**:
- 1 Epic (grid-f21b)
- 9 Feature issues (one per phase)
- 7 Task issues (Phase 1: 4 tasks, Phase 2: 3 tasks)

**Total**: 17 beads issues

**Next Steps**:
1. Review epic and phase descriptions
2. Start with Phase 1 tasks (grid-82b8, grid-8673, grid-c734, grid-a741)
3. Create additional tasks as needed during implementation
4. Update task status as work progresses
