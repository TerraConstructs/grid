We will implement state dependencies by only updating the proto contracts (no protobuf version bump, this is alpha and contract is not frozen, breaking changes are acceptable) and implementing the apiserver handlers. The system should allow clients to declare dependencies, list dependencies and dependents, search by output key, and provide a topological ordering of states. Each dependency is a directed edge from one state's output key to another state (multigraph). The system must enforce acyclicity, support multiple edges per state pair, and allow optional mock outputs when the producer output does not yet exist. State statuses are always computed on demand from edge statuses (using output fingerprints parsed out of tfstate updates). The CLI should gain new commands under gridctl deps command group to add, remove, list, search, and sync to update managed Terraform locals blocks. Generated HCL files should include terraform_remote_state data sources keyed by state GUIDs and a locals block mapping outputs to variables, with support for per-edge to_input overrides to customize local names. In Phase 1, the system should compute full graphs in memory/responses; pagination and filtering can be deferred to later phases. The apiserver should only return the necessary data for the client to generate the HCL files (the HCL templates kept at the gridctl, using embedFS similar to existing `backend.tf` template)

Research points:
1. RDBMS patterns for directed acyclic graphs (DAGs) and multigraphs + Go graph libraries for in-memory validation and toposort
2. TF State parse outputs on update

1. Research RDBMS way to model directed **acyclic multigraphs**

## 1) “Edge table” (adjacency list) — the general workhorse

* **Tables:** `states(id …)`, `edges(id, src_id, dst_id, kind, weight, …)`.
* Works for **multigraphs** (allow parallel edges) by giving each edge its own PK.
* Query traversal with **recursive CTEs**; Postgres evaluates these iteratively and they're well-documented.

### Enforce “Acyclic”

Sample **BEFORE INSERT/UPDATE** trigger on `edges` that rejects an edge if `dst` already reaches `src` (should include composite uniqueness based on from_state, from_output):

```sql
CREATE OR REPLACE FUNCTION prevent_cycle() RETURNS trigger AS $$
BEGIN
  -- is there already a path from NEW.dst -> NEW.src ?
  IF EXISTS (
    WITH RECURSIVE r(n) AS (
      SELECT NEW.dst
      UNION ALL
      SELECT e.dst_id FROM edges e JOIN r ON e.src_id = r.n
    )
    SELECT 1 FROM r WHERE n = NEW.src
  ) THEN
    RAISE EXCEPTION 'cycle detected % -> %', NEW.src_id, NEW.dst_id;
  END IF;
  RETURN NEW;
END$$ LANGUAGE plpgsql;

CREATE TRIGGER edges_cycle
BEFORE INSERT OR UPDATE ON edges
FOR EACH ROW EXECUTE FUNCTION prevent_cycle();
```

(The pattern of using a recursive CTE or trigger to **detect/prevent cycles** is a known approach in Postgres.)

**Pros:** Simple, flexible (fits DAGs and multigraphs).
**Cons:** Longer paths can be slower to query than specialized patterns; you tune with indexes on `(src_id)`, `(dst_id)` and (optionally) `(src_id, dst_id)`.

## 2) Closure table — fast ancestor/descendant queries

* **Tables:** `paths(ancestor_id, descendant_id, depth)` alongside your base edge table.
* On insert of an edge, insert all implied paths `(A -> … -> B)`. Queries become trivial joins. Good when you **read traversals a lot**.

**Cons:** Extra write-amplification; still add the cycle-prevention check (or maintain with triggers that refuse paths that would create a loop).

## 3) Materialized-path with `ltree` (when it’s *tree-ish*)

* Postgres’ `ltree` extension stores **label paths** with GiST indexes and rich operators. Great for strict trees or tree-like data (org charts, folders). You *can* approximate DAGs with multiple paths per node, but it’s not ideal for general DAGs.

There are writeups showing **acyclic directed graphs** implemented with `ltree` + triggers if your structure is close to hierarchical.

---

# Recommended Go tooling (2025 “state of the practice”)

**Pick your ORM/query layer** (all work great with Postgres) and add an in-memory graph lib for validation/toposort as needed.

## Orms / SQL mappers that play nicely with recursive CTEs

* **Bun** (SQL-first ORM). Has first-class **CTE** support (`With(...)`) which makes recursive queries straightforward. Great fit for adjacency-list + cycle-check trigger + traversal queries.

## Graph algorithms in Go (in-memory)

* **Gonum/graph** — canonical Go graph interfaces and algorithms; includes **multigraph** types (`graph/multi`) and **topological sort** helpers (`graph/topo`) to validate DAGs pre-write and to compute execution order. Use this in your service layer; persist edges in Postgres.

---

# Concrete, production-friendly schema (covers DAG + multigraph)

```sql
-- existing states table <omit for brevity>

-- Multigraph-friendly: every edge has its own PK; optional 'kind' to tag parallels
CREATE TABLE edges (
  id     BIGSERIAL PRIMARY KEY,
  from_state GUID NOT NULL REFERENCES states(guid) ON DELETE CASCADE,
  from_output TEXT   NOT NULL,  -- which output key from from_state
  to_state GUID NOT NULL REFERENCES states(guid) ON DELETE CASCADE,
  kind   TEXT   NOT NULL DEFAULT 'default',
  weight DOUBLE PRECISION,
  props  JSONB,
  UNIQUE (from_state, from_output, to_state)      -- relax/remove if true parallelism is desired
);

CREATE INDEX ON edges (src_id);
CREATE INDEX ON edges (dst_id);
```

* Add the **cycle-prevention trigger** from above to guarantee acyclicity at the DB boundary. (Pattern backed by long-standing Postgres guidance on using recursive queries/triggers for cycle detection.)
* If you need **speedy reachability queries**, add a **closure table**:

```sql
CREATE TABLE paths (
  ancestor_id  BIGINT NOT NULL REFERENCES states(guid) ON DELETE CASCADE,
  descendant_id BIGINT NOT NULL REFERENCES states(guid) ON DELETE CASCADE,
  depth        INT NOT NULL,
  PRIMARY KEY (ancestor_id, descendant_id)
);
```

Maintain `paths` in the same transaction when inserting/deleting edges (CTEs help).

---

# How this looks in Go (pseudocode)

## Using Bun (CTE-first)

```go
// Validate with Gonum first (optional), then write in one TX:
tx, _ := db.BeginTx(ctx, nil)
defer tx.Rollback()

// Example: insert edge with cycle check enforced by DB trigger
_, err := tx.NewInsert().Model(&Edge{SrcID: src, DstID: dst, Kind: kind}).Exec(ctx)
if err != nil { /* handle "cycle detected" */ }

// Optional: maintain closure table via a recursive CTE in the same tx:
_, err = tx.NewInsert().
  With("anc", tx.NewSelect().
      TableExpr("paths").
      Where("descendant_id = ?", src).
      Column("ancestor_id, ? AS descendant_id, depth + 1", dst).
      UnionAll(
        tx.NewSelect().TableExpr("VALUES (?, ?, 0)", src, dst).
        ColumnExpr("column1, column2, column3"),
      ),
  ).
  TableExpr("paths").
  Column("ancestor_id", "descendant_id", "depth").
  Exec(ctx)
```

(CTE support shown in Bun's docs.) https://bun.uptrace.dev/guide/query-common-table-expressions.html

## Validating order / cycles in memory

If you already have a batch of edges in memory, run a **topological sort** to fail fast:

```go
order, err := topo.Sort(myDirectedGraph) // from gonum.org/v1/gonum/graph/topo
// err != nil => cycle; list is in err.(topo.Unorderable).Cycles
```

Use `graph/multi` for **multigraphs** in Go if you need parallel edges as proper first-class lines.

---

# When to pick which pattern

* **Most systems (DAG or multigraph):** Edge table + recursive CTEs, with a **DB-level cycle trigger**. Add a **closure table** only if reachability/ancestors/descendants are hot paths. 

---

## Shortlist (2025)

* **Bun** — SQL-first ORM with CTEs and great Postgres support.
* **Gonum/graph** — robust algorithms + **multigraph** + **toposort**.

Reference golang graphing libraries:
- https://pkg.go.dev/github.com/dominikbraun/graph
- https://pkg.go.dev/gonum.org/v1/gonum/graph/multi

2. Parse outputs from TF State on update

Example apiserver logic:

Attribution: https://github.com/diggerhq/digger/blob/78111a6652e1eaac45be7520cda6d4412430c0e9/taco/internal/deps/graph.go

```go
// StateGraph is memory representation of the full dependency graph
type StateGraph struct {
  // to be defined
}

// Collect edges and build adjacency
type Edge struct {
    EdgeID       string
    FromUnit     string
    FromOutput   string
    ToUnit       string
    InDigest     string
    OutDigest    string
    Status       string
    LastInAt     string
    LastOutAt    string
}

// TFState models the minimal Terraform 1.x state structure we need
type TFState struct {
    Serial    int         `json:"serial"`
    Lineage   string      `json:"lineage"`
    Resources []TFResource `json:"resources"`
}

type TFResource struct {
    Mode      string        `json:"mode"`
    Type      string        `json:"type"`
    Name      string        `json:"name"`
    Provider  string        `json:"provider"`
    Instances []TFInstance  `json:"instances"`
}

type TFInstance struct {
    Attributes map[string]interface{} `json:"attributes"`
}

// TFOutputs is a small view of TF state outputs
type TFOutputs struct {
    Outputs map[string]struct{ Value interface{} `json:"value"` } `json:"outputs"`
}

// Use for unmarshaling TFState outputs only...

    // Parse new tfstate outputs once
    var outs TFOutputs
    _ = json.Unmarshal(newTFState, &outs) // if this fails, outs.Outputs stays nil
```

Sample logic to update edges on state writes (from OpenTaco project) - pseudocode, depends on Research above if used...

```golang
// UpdateGraph updates dependency edges in the graph tfstate in response to a write
// to a State with content newTFState (json data). It performs both outgoing (source refresh) and incoming
// (target acknowledge) updates in a single locked read-modify-write cycle.
func UpdateGraph(ctx context.Context, repo *StateRepository, stateGuid string, newTFState []byte) {
    // Fast exits: graph unit must exist and be lockable. Never fail the caller's write.
    // Acquire lock for edges update
    lock := &repo.LockGraphInfo{ID: fmt.Sprintf("deps-%d", time.Now().UnixNano()), Who: "opentaco-deps", Version: "1.0.0", Created: time.Now()}
    if err := repo.LockGraphInfo(ctx, lock); err != nil {
        // Graph missing or locked by someone else — skip quietly
        return
    }
    defer func() { _ = repo.UnlockGraphInfo(ctx, lock.ID) }()

    // Read current graph from states, outputs and edges
    var graph StateGraph
    if err := repo.GetStateGraph(ctx, &graph); err != nil {
        return
    }

    // Parse new tfstate outputs once
    var outs TFOutputs
    _ = json.Unmarshal(newTFState, &outs) // if this fails, outs.Outputs stays nil

    now := time.Now().UTC().Format(time.RFC3339)

    changed := false
    // iterate graph based on incoming state data (see research on RDBMS DAG patterns and deserialization)
    for i := range graph.Edges {
        edge := &graph.Edges[i]

        fromID := edge.FromStateGUID
        toID := edge.ToStateGUID
        fromOut := edge.FromOutput

        // Ensure fields exist to avoid nil panics when writing back
        # _ = getString(attrs["in_digest"]) // probe
        # _ = getString(attrs["out_digest"]) // probe
        status := edge.Status // may be empty

        // A) Outgoing (source refresh)
        if fromID == stateGuid {
            // If from_output exists in new state outputs, recompute in_digest and status
            if outs.Outputs != nil {
                if ov, ok := outs.Outputs[fromOut]; ok {
                    dig := digestValue(ov.Value)
                    if edge.InDigest != dig {
                        edge.InDigest = dig
                        edge.LastInAt = now
                        changed = true
                    }
                    // Recompute status relative to out_digest
                    outD := edge.OutDigest
                    newStatus := "pending"
                    if dig != "" && outD != "" && dig == outD {
                        newStatus = "ok"
                    }
                    if status != newStatus {
                        edge.Status = newStatus
                        changed = true
                    }
                } else {
                    // Source output missing
                    if status != "unknown" {
                        edge.Status = "unknown"
                        changed = true
                    }
                }
            } else {
                // Cannot parse outputs, set unknown
                if status != "unknown" {
                    edge.Status = "unknown"
                    changed = true
                }
            }
        }

        // B) Incoming (target acknowledge)
        if toID == stateGuid {
            inD := edge.InDigest
            if inD != "" {
                if edge.OutDigest != inD {
                    edge.OutDigest = inD
                    edge.LastOutAt = now
                    edge.Status = "ok"
                    changed = true
                } else if status != "ok" {
                    edge.Status = "ok"
                    changed = true
                }
            }
        }
    }

    if !changed {
        return
    }

    // Bump serial and write back
    graph.Serial++
    // Pass lockID to satisfy write while locked
    _ = repo.UpdateStateGraph(ctx, graph, lock.ID)
}

// ComputeStateStatus reads the graph tfstate and returns the status payload for a given StateID.
// If the graph is missing/corrupt, it returns a best-effort empty green status.
func ComputeStateStatus(ctx context.Context, repo *StateRepository, stateGuid string) (*StateStatus, error) {
    // Read current graph from states, outputs and edges
    var st StateGraph
    if err := repo.GetStateGraph(ctx, &graph); err != nil {
        // Treat missing graph as no edges
        return &StateStatus{StateID: unitID, Status: "green", Incoming: nil, Summary: Summary{}}, nil
    }


    var edges []edge
    adj := map[string][]string{}
    incoming := []IncomingEdge{}
    for _, edge := range graph.Edges {
        # frm := edge.FromStateGUID
        # to := edge.ToStateGUID
        # e := Edge{
        #     EdgeID:    edge.ID,
        #     FromStateGUID: frm,
        #     FromOutput: edge.FromOutput,
        #     ToStateGUID:   to,
        #     InDigest:  edge.InDigest,
        #     OutDigest: edge.OutDigest,
        #     Status:    edge.Status,
        #     LastInAt:  edge.LastInAt,
        #     LastOutAt: edge.LastOutAt,
        # }
        edges = append(edges, e)
        adj[frm] = append(adj[frm], to)

        if to == stateGuid {
            incoming = append(incoming, IncomingEdge{
                EdgeID: e.EdgeID,
                FromUnitID: e.FromUnit,
                FromOutput: e.FromOutput,
                Status: e.Status,
                InDigest: e.InDigest,
                OutDigest: e.OutDigest,
                LastInAt: e.LastInAt,
                LastOutAt: e.LastOutAt,
            })
        }
    }

    // Determine red units: any unit with an incoming edge pending.
    red := map[string]bool{}
    for _, e := range edges {
        if e.Status == "pending" {
            red[e.ToUnit] = true
        }
    }

    // Propagate yellow from red upstreams
    yellow := map[string]bool{}
    // BFS from all red units
    q := make([]string, 0, len(red))
    seen := map[string]bool{}
    for s := range red {
        q = append(q, s)
        seen[s] = true
    }
    for len(q) > 0 {
        cur := q[0]
        q = q[1:]
        for _, nxt := range adj[cur] {
            if seen[nxt] { continue }
            yellow[nxt] = true
            seen[nxt] = true
            q = append(q, nxt)
        }
    }

    // Compute incoming summary for target
    sum := Summary{}
    for _, ie := range incoming {
        switch ie.Status {
        case "ok": sum.IncomingOK++
        case "pending": sum.IncomingPending++
        default: sum.IncomingUnknown++
        }
    }

    // Determine target status
    stStatus := "green"
    if red[stateGuid] {
        stStatus = "red"
    } else if yellow[stateGuid] {
        stStatus = "yellow"
    }

    // Sort incoming edges by status for stable output
    sort.Slice(incoming, func(i, j int) bool {
        if incoming[i].Status == incoming[j].Status {
            return incoming[i].FromUnitID < incoming[j].FromUnitID
        }
        // pending first, then unknown, then ok
        order := map[string]int{"pending": 0, "unknown": 1, "ok": 2}
        return order[incoming[i].Status] < order[incoming[j].Status]
    })

    return &StateStatus{StateID: stateGuid, Status: stStatus, Incoming: incoming, Summary: sum}, nil
}

// Types for API response
type StateStatus struct {
    StateID string         `json:"state_id"`
    Status  string         `json:"status"`
    Incoming []IncomingEdge `json:"incoming"`
    Summary Summary        `json:"summary"`
}

type IncomingEdge struct {
    EdgeID       string `json:"edge_id,omitempty"`
    FromStateGUID   string `json:"from_state_id"`
    FromOutput   string `json:"from_output"`
    Status       string `json:"status"`
    InDigest     string `json:"in_digest,omitempty"`
    OutDigest    string `json:"out_digest,omitempty"`
    LastInAt     string `json:"last_in_at,omitempty"`
    LastOutAt    string `json:"last_out_at,omitempty"`
}

type Summary struct {
    IncomingOK      int `json:"incoming_ok"`
    IncomingPending int `json:"incoming_pending"`
    IncomingUnknown int `json:"incoming_unknown"`
}

// digestValue computes SHA-256 over canonical JSON bytes and returns base58 string
func digestValue(v interface{}) string {
    b := canonicalJSON(v)
    if b == nil {
        return ""
    }
    h := sha256.Sum256(b)
    return base58.Encode(h[:])
}

// canonicalJSON produces a deterministic JSON encoding for a limited set of values
func canonicalJSON(v interface{}) []byte {
    // Handle common Terraform output shapes: nil, bool, float64, string, []any, map[string]any
    switch t := v.(type) {
    case nil:
        return []byte("null")
    case bool:
        if t { return []byte("true") } 
        return []byte("false")
    case float64:
        // Use json.Marshal for numbers which is stable for float64
        b, _ := json.Marshal(t)
        return b
    case int, int64, uint64, json.Number:
        b, _ := json.Marshal(t)
        return b
    case string:
        b, _ := json.Marshal(t)
        return b
    case []interface{}:
        // Arrays in order
        var out []byte
        out = append(out, '[')
        for i, el := range t {
            if i > 0 { out = append(out, ',') }
            out = append(out, canonicalJSON(el)...)
        }
        out = append(out, ']')
        return out
    case map[string]interface{}:
        // Sort keys lexicographically
        keys := make([]string, 0, len(t))
        for k := range t { keys = append(keys, k) }
        sort.Strings(keys)
        var out []byte
        out = append(out, '{')
        for i, k := range keys {
            if i > 0 { out = append(out, ',') }
            kb, _ := json.Marshal(k)
            out = append(out, kb...)
            out = append(out, ':')
            out = append(out, canonicalJSON(t[k])...)
        }
        out = append(out, '}')
        return out
    default:
        // Attempt to coerce via JSON first
        var m map[string]interface{}
        if b, err := json.Marshal(t); err == nil {
            if err := json.Unmarshal(b, &m); err == nil {
                return canonicalJSON(m)
            }
        }
        // As a last resort, encode via json.Marshal
        b, _ := json.Marshal(t)
        return b
    }
}
```
