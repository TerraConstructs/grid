# State dimensions - scope reduction

This document outlines a minimal, dependency-free approach to implement state labels with validation and filtering, based on your functional requirements (FRs). It avoids heavy dependencies like k8s apimachinery or JSON Schema libraries, focusing on simplicity and maintainability.

Focus is on following areas to reduce scope:

- Drop json-schema as validation mechanism
- Drop EAV, advanced query expression to SQL translation
- No existing table data or migrations for adoption of state dimensions
- No dimension key aliasing, coercion (closest match), or deprecation warnings

Terminology & field names

Pick one and stick to it across API/DB/UI. My take:

* Use **labels** for the *map* (Kubernetes familiarity, concise).
* Reserve **metadata** for “everything else about the state” (timestamps, lock, etc.).
* CLI: `gridctl state set/list` with flag `-l,--label` feels natural. (gridctl state set --label team=foo --label project=sentry)

If you prefer the cloud vibe, **tags** also works; below I’ll say **labels** (swap names if you choose tags)

Terminology conclusion:
- "labels" = user-provided key-value pairs (map[string]any) - instead of "tags" or "dimensions"
- "policy" = validation rules for labels (allowed keys, value constraints, caps, reserved prefixes)


## Bexpr filtering

The plan is to do “bexpr-on-the-server”, solid for ~1k states using its existing grammar

https://github.com/hashicorp/go-bexpr/blob/main/grammar/grammar.peg (defined in pigeon to manage the grammar parser to AST in go) and we should not modify it (just keep as is) - which should be documented in the API docs.

Here’s a crisp way to wire it up with HashiCorp go-bexpr (https://github.com/hashicorp/go-bexpr) + a labels JSON column, filtering entirely in memory (no SQL push-down yet).

### Protobuf changes (typed labels + bexpr filter)

typed values (FR-008a) and an in-memory bexpr filter string. Add:

```proto
// Typed label value.
message LabelValue {
  oneof kind {
    string s = 1;
    double n = 2;  // numbers (int/float accepted by JSON -> double)
    bool   b = 3;
  }
}

// Labels patch: add/update/remove
message LabelsPatch {
  map<string, LabelValue> upsert = 1; // set or update
  repeated string remove = 2;         // delete these keys
}

// ----- New APIs (optional names; keep simple) -----

message UpdateStateLabelsRequest {
  oneof state {
    string logic_id = 1;
    string guid = 2;
  }
  LabelsPatch patch = 3;
}

message UpdateStateLabelsResponse {
  string guid = 1;
  map<string, LabelValue> labels = 2;
}

// ListStates: add filter + projection controls
message ListStatesRequest {
  // bexpr expression – MUST use go-bexpr grammar; server treats empty as match-all.
  string filter = 1;

  // Return labels with each item (default true).
  optional bool include_labels = 2;

  // Paging
  int32 page_size = 3;      // default/max: 1000 (fits in-memory plan)
  string page_token = 4;
}

message StateInfo {
  string guid = 1;
  string logic_id = 2;
  bool locked = 3;
  google.protobuf.Timestamp created_at = 4;
  google.protobuf.Timestamp updated_at = 5;
  int64 size_bytes = 6;

  optional string computed_status = 7;
  repeated string dependency_logic_ids = 8;

  // Optional typed labels projection (controlled by include_labels)
  map<string, LabelValue> labels = 9;
}
```

* **Why not `Struct`?** You need type discrimination + size cap; `LabelValue` is explicit and compact.

### Storage model (Postgres + SQLite)

Option A (recommended now): **JSON column on `states`**

* Simpler to fetch ~1000 rows and filter in memory via bexpr.
* Schema (Bun migration defined in Go)

```sql
-- Postgres
ALTER TABLE states ADD COLUMN labels jsonb NOT NULL DEFAULT '{}'::jsonb;
CREATE INDEX IF NOT EXISTS idx_states_labels ON states USING gin (labels jsonb_path_ops);

-- SQLite (future ref)
ALTER TABLE states ADD COLUMN labels TEXT NOT NULL DEFAULT '{}'; -- JSON1 present
```

> If you expect >10–20k states soon, keep an **EAV side table** ready for push-down later. No change to wire contract.


### Repo projection & in-memory filtering with bexpr

### SQL projection (PG & SQLite)

Fetch the **minimal set** (omit `state_content` for list views) and include the `labels` JSON blob.

```sql
-- Postgres
SELECT s.guid, s.logic_id, s.locked, s.created_at, s.updated_at, s.labels
FROM states s
ORDER BY s.updated_at DESC
LIMIT $1 OFFSET $2;

-- SQLite
SELECT s.guid, s.logic_id, s.locked, s.created_at, s.updated_at, s.labels
FROM states s
ORDER BY s.updated_at DESC
LIMIT ? OFFSET ?;
```

### Go: bexpr evaluation

* **Context**: pass the **labels as a flat map** (Option A). This lets expressions use identifiers directly (`team == "foo"`).

  [NEEDS CLARIFICATION] “identifier-safe” keys verify if allowed `.` `/` `-` `_` or filtering requires `[A-Za-z_][A-Za-z0-9_]*` keys; store original as-is for display (given bexpr grammar rules and structpointer functionality).

```go
import bexpr "github.com/hashicorp/go-bexpr"

type row struct {
  GUID       string          `bun:"guid"`
  LogicID    string          `bun:"logic_id"`
  Locked     bool            `bun:"locked"`
  CreatedAt  time.Time       `bun:"created_at"`
  UpdatedAt  time.Time       `bun:"updated_at"`
  LabelsJSON json.RawMessage `bun:"labels"` // confirm this
}

func compileFilter(filter string) (*bexpr.Evaluator, error) {
  if filter == "" { return nil, nil }
  return bexpr.CreateEvaluator(filter)
}

func match(ev *bexpr.Evaluator, ctx map[string]any) (bool, error) {
  if ev == nil { return true, nil }
  return ev.EvaluateBoolean(ctx)
}

func labelsToContext(raw json.RawMessage) map[string]any {
  // unmarshal into map[string]any preserving number/bool types
  var m map[string]any
  _ = json.Unmarshal(raw, &m)
  if m == nil { m = map[string]any{} }
  return m
}
```

Apply:

```go
ev, err := compileFilter(req.Filter) // return 400 if grammar invalid
rows := repo.ListProjection(...)
for _, r := range rows {
  ctx := labelsToContext(r.LabelsJSON) // {"team":"foo","project":"sentry","size":3,"active":true}
  ok, err := match(ev, ctx)
  if err != nil { /* 400 invalid eval */ }
  if !ok { continue }
  // include in response, optionally map to LabelValue for type-safe proto
}
```

**Example filters (bexpr grammar unchanged):**

* `team in ["foo","bar"] && project == "sentry"`
* `environment != "prod" || size >= 3`
* `active == true && (region == "ap-southeast-1" || region == "us-east-1")`

NOTE: If bexpr grammar does not allow `.` or `-` in identifiers, ensure this is documented and enforced in the label key policies (e.g., use `_` instead).

Guardrails:

* Limit filter length (e.g., **4 KB**).
* Optionally cap array literal length (e.g., ≤ 100).

See further notes on LabelPolicy and normalization below.

### CLI surface

* Get/show: `gridctl state get <logic_id>` includes labels option.
* Set single: `gridctl state set <logic_id> --label team=foo --label project=sentry --label active=true --label -color` (add and remove using `-key` syntax).
* List with filter:
  `gridctl state list --filter 'team in ["foo","bar"] && project == "sentry"'`
* List with simple label filter (syntactic sugar):
    `gridctl state list --label team=foo`

The CLI just sends the **filter string** as-is (no grammar rewriting) or builds string from repeated --label string slice (and).

---

### Future: push-down (no wire change)

When scale grows, you can add an internal translator for a **safe subset** of bexpr → SQL:

* `==`, `!=`, `in`, `>=`, `<=`, `>`, `<`, `&&`, `||`, parentheses.
* For Postgres JSONB:

  * equality: `(labels->>'team') = $1`
  * membership: `(labels ? 'team')` (exists)
  * numeric comparisons: `((labels->>'size')::numeric) >= $1`
  * boolean: `(labels->>'active')::boolean = true`
  * GIN + expression indexes for hot keys.

Fallback: if parse → SQL exceeds complexity caps, evaluate in memory (same filter string).

## Tiny `labelpolicy` (regex + small maps) — dead simple

### Validation + normalization pipeline (write path)

* **Step 0**: enforce FR-008/008a/008b (key regex, type set, caps).
* **Step 1**: normalize (case, trim).
* **Step 2**: **key resolution** if exact match → OK
* **Step 3**: **value resolution** (per key)

  * type check (string/number/bool).
  * if enum defined:
    * exact in enum → OK
  * if enum not defined: allow
* **Step 4**: limits (≤32 keys, length checks).
* **Step 5**: return **normalized** labels for storage.

Basic validation pipeline (FR-008 series) exmple

Run this anywhere labels are **created/updated** (CreateState, UpdateStateLabels, bulk import):

```go
var keyRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._:/-]{0,31}$`)

type LabelKV struct {
  Key string
  Val any // string | float64 | bool (from JSON)
}

func validateLabels(current map[string]any, patchUpserts map[string]any, removals []string) error {
  // 1) validate keys
  for k := range patchUpserts {
    if !keyRE.MatchString(k) { return fmt.Errorf("invalid key %q", k) }
    // reserved namespaces (example)
    if strings.HasPrefix(k, "grid.io/") { return fmt.Errorf("reserved namespace: %q", k) }
  }
  // 2) validate value types + size
  for k, v := range patchUpserts {
    switch vv := v.(type) {
    case string:
      if len(vv) > 256 { return fmt.Errorf("value too long for %q", k) }
    case float64: // JSON numbers -> float64
    case bool:
    default:
      return fmt.Errorf("unsupported value type for %q", k)
    }
  }
  // 3) enforce cap (≤ 32 keys)
  next := maps.Clone(current)
  for _, k := range removals { delete(next, k) }
  for k, v := range patchUpserts { next[k] = v }
  if len(next) > 32 { return fmt.Errorf("label cap exceeded: %d > 32", len(next)) }

  return nil
}
```

> consider per-key allowed value sets, kept “schema” (map of `key -> enum/type`) in memory and validate against it. without full JSON-Schema.

### Functional Requirements recap → minimal policy

* **FR-008** keys: lowercase alnum up to 32 chars, allow `- _ / : .`
* **FR-008a** values: string | number | bool
* **FR-008b** cap: ≤ 32 keys
* Optional: `allowedValues` per key (enum)

### Minimal types

```go
type Policy struct {
    AllowedKeys     map[string]struct{}            // nil/empty => any key allowed by regex
    AllowedValues   map[string]map[string]struct{} // key -> set of allowed strings (optional)
    ReservedPrefixes []string                       // e.g., {"kubernetes.io/", "grid.io/internal/"}
    MaxKeys         int                             // e.g., 32
    MaxValueLen     int                             // e.g., 256
}

var (
    // ^[a-z0-9][a-z0-9._:/-]{0,31}$  -> lowercase, 1..32, allowed punctuation
    keyRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._:/-]{0,31}$`)
)
```

### Validator

```go
// labels is the user-provided JSON decoded as map[string]any
func (p *Policy) Validate(labels map[string]any) error {
    if labels == nil {
        return nil
    }
    if p.MaxKeys > 0 && len(labels) > p.MaxKeys {
        return fmt.Errorf("label cap exceeded: %d > %d", len(labels), p.MaxKeys)
    }

    for k, v := range labels {
        if !keyRE.MatchString(k) {
            return fmt.Errorf("invalid key %q: must match %s", k, keyRE.String())
        }
        for _, pref := range p.ReservedPrefixes {
            if strings.HasPrefix(k, pref) {
                return fmt.Errorf("reserved namespace for key %q (prefix %q)", k, pref)
            }
        }
        if len(p.AllowedKeys) > 0 {
            if _, ok := p.AllowedKeys[k]; !ok {
                return fmt.Errorf("unknown key %q; allowed keys: %s", k, joinKeys(p.AllowedKeys))
            }
        }

        switch vv := v.(type) {
        case string:
            if p.MaxValueLen > 0 && len(vv) > p.MaxValueLen {
                return fmt.Errorf("value too long for %q (max %d)", k, p.MaxValueLen)
            }
            if allowed, ok := p.AllowedValues[k]; ok {
                if _, ok := allowed[vv]; !ok {
                    return fmt.Errorf("invalid value %q for %s; allowed: %s", vv, k, joinKeys(allowed))
                }
            }
        case float64, bool: // JSON decoder uses float64 for numbers
            // If you want enums for numbers/bools too, add maps[string]set[fmtVal]
            // Otherwise accept any number/bool.
        default:
            return fmt.Errorf("unsupported type for %q; allowed: string|number|bool", k)
        }
    }
    return nil
}

func joinKeys(m map[string]struct{}) string {
    out := make([]string, 0, len(m))
    for k := range m { out = append(out, k) }
    sort.Strings(out)
    return strings.Join(out, ",")
}
```

# Proto messages to manage Labels policy

To Be Defined (i.e. GetLabelPolicy -> return json doc, UpdateLabelPolicy -> accept json doc, ValidateLabels -> dry-run a set of labels and returns normalized output + warnings, GetLabelEnum -> return allowed values for UI pickers - cached).


# Integration with bexpr filters

* You store labels as JSON, validated once on write with the `Policy` above.
* For listing, you project rows and evaluate the **bexpr** filter against a `map[string]any` context. No extra deps.

No change needed. Your **stored labels are normalized**, so filters like:

```
team in ["platform","data"] && env == "production"
```

behave predictably. You can also expose policy enums to the CLI for local validation/autocomplete.

# Performance

* The Label Policy evaluator is O(#labels) with only hash lookups—microseconds per state.
* For list queries you’re already evaluating bexpr on ~1k states; this validation isn’t in the hot path unless you also validate filters (not needed).

### Sample test mock data


```go
var DefaultPolicy = &Policy{
    AllowedKeys:      nil, // nil => any key allowed if it matches keyRE
    AllowedValues:    map[string]map[string]struct{}{
        "env":    set("production","staging","development"),
        "team":   set("platform","data","security"),
        "project":set("sentry","hulk","thor"),
    },
    ReservedPrefixes: []string{"grid.io/internal/"},
    MaxKeys:          32,
    MaxValueLen:      256,
}

func set(ss ...string) map[string]struct{} {
    m := make(map[string]struct{}, len(ss))
    for _, s := range ss { m[s] = struct{}{} }
    return m
}
```

## Migration & safety considerations

(avoid if YAGNI/premature optimization)

* Ship policy in **warn-only** mode first (log + client warnings).
* After 1–2 weeks, flip to **enforce** (reject).
* Provide a bulk **normalize** tool:

  ```
  gridctl state labels normalize --all --dry-run
  gridctl state labels normalize --all --write
  ```
* Keep **policy versions**; store version on each write for audit.

### Normalization (optional but useful)

If you want to be extra typo-proof without heavy “aliasing”:

* **Lowercase keys** on write (you already require lowercase).
* Optionally `strings.TrimSpace` on string values.
* If you want to accept `prod` but store `production` *later*, add a tiny map for **value remap** (not a trie, not fuzzy):

```go
// Optional tiny remap if you choose:
ValueRemap map[string]map[string]string // key -> (input -> canonical)
```

Keep it empty until users ask for it.

### Using with bexpr (unchanged)

* You store labels as JSON, validated once on write with the `Policy` above.
* For listing, you project rows and evaluate the **bexpr** filter against a `map[string]any` context. No extra deps.


## Summary

* Start with a purpose-built **Label Policy**: allowed keys, enums, limits. It’s simple, fast, and solves 95% of typo problems (tema instead of team).
* Consider **optional JSON Schema** for orgs that want broader constraints down the line
* Normalize on write so bexpr queries stay clean.
* Provide friendly suggestions and a dry-run to ease adoption.

That’s all you need to get the feature out, collect feedback, and iterate—**no extra deps**, fully aligned with your FRs, and perfectly compatible with your **bexpr** filtering path.
