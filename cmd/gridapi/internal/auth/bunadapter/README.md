
# Forked Adapter

Internal fork of the [Bun Casbin adapter](https://github.com/msales/casbin-bun-adapter/blob/v1.0.7/adapter.go) so the CasbinRule model no longer hard-codes the `casbin.` schema, keeping the original API but allowing the same table name to work in Postgres (public search path) and SQLite.

## Bug Fix: Support Empty Fields

Comprehensive Review of `bunadapter/adapter.go`

Methods Reviewed:

1. LoadPolicy() (lines 44-57) ✅
   -  Uses String() method - now fixed
2. RemoveFilteredPolicy() (lines 122-149) ✅ CORRECT
   -  Skips empty values in WHERE clauses - appropriate behavior
3. LoadFilteredPolicy() (lines 153-190) ✅
   -  Uses String() method - now fixed
   -  Uses buildQuery() - correct behavior
4. UpdateFilteredPolicies() (lines 238-297) ✅ NOW FIXED
   -  Uses toStringPolicy() - now fixed to preserve empty fields
5. buildQuery() (lines 365-387) ✅ CORRECT
   -  Skips empty values for SQL filters - appropriate behavior
6. String() (lines 432-464) ✅ FIXED
   -  Converts to CSV string for Casbin
   -  Now preserves empty fields
7. QueryWhereGroup() (lines 467-489) ✅ CORRECT
   -  Skips empty values in SQL WHERE clauses - appropriate behavior
8. toStringPolicy() (lines 491-516) ✅ FIXED
   -  Returns policy as string slice
   -  Now preserves empty fields

Two methods fixed:
1. String() - CSV serialization for Casbin parser
2. toStringPolicy() - String slice serialization for Casbin API

**The Bug**: The methods were skipping empty string fields when building the CSV string for Casbin, causing field position misalignment when parsing policies with empty
scopeExpr values.

**The Fix**: Modified the methods to preserve empty fields up to the last non-empty field, ensuring Casbin correctly parses all 5 fields defined in the model: `p = role, obj, act, scopeExpr, eft`.

This now allows wildcard policies like `role:platform-engineer, *, *, , allow` to work correctly!
