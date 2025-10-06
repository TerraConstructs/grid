# Tech Debt: UX Improvements feature

## Code Quality Improvements

- [X] Context read/write for gridctl should be internal/context package so cmd/deps doesn't depend on cmd/state
- [ ] New BunRepository should return the interface, not the concrete type. this ensures the type satisfies the interface contract - i.e. func NewBunStateRepository(db *bun.DB) *BunStateRepository should return StateRepository.

### TODOs from code review

- [X] deps topo doesn't use context yet

### TFparser entry points

We have both ParseState (keys+values) and ParseOutputs/ParseOutputKeys (values or keys only). That’s fine, but consider standardizing call sites:
- Keep ParseState for write paths (you already do).
- Keep ParseOutputKeys for read fallback (you already do).

Optionally, update EdgeUpdateJob.UpdateEdges to call ParseState and use .Values for consistency; not required since handlers already use UpdateEdgesWithOutputs.


### Better caching of output fingerprints to avoid reparsing on every edge initialization

The cache only stores keys and sensitivity, not values or fingerprints. For initializing InDigest you need the value (or a precomputed digest), 

Add a digest column to the cache and populate it transactionally on writes:

1. Schema: add value_digest TEXT NOT NULL DEFAULT '' to state_outputs (or nullable).
2. Write path: state/service.go:167 already has parsed.Values. Compute ComputeFingerprint(parsed.Values[k]) for each key and pass it to the repo. Update UpdateContentAndUpsertOutputs to insert/update the digest alongside key/sensitive/serial in the same tx (cmd/gridapi/internal/repository/bun_state_repository.go:95).
3. Read path: in initializeEdgeIfProducerHasOutput, read the single row (state_guid, output_key) from cache and set edge.InDigest = value_digest directly. No reparse needed, and still fully consistent with the write transaction.

Why this is safe and consistent

- Atomicity: Writes already update state content and cache together in one DB transaction (cmd/gridapi/internal/repository/bun_state_repository.go:95). Including digest in that upsert preserves the same guarantee for fingerprints.
- Security: Digests are already stored on edges. Adding them to state_outputs doesn’t increase exposure via RPC since you don’t return digests. If you want extra caution, only persist digests for non-sensitive outputs, or persist for all but never expose via any API.

### Edge integration testing

high‑value scenarios to add to tests/integration/context_aware_test.go to catch timing, cache, and lifecycle issues around edges, outputs, and digests.

Edge Timing

Producer Has Output Before Edge
Steps: Producer posts outputs (serial 1) → add edge → consumer posts state.
Assert: edge.InDigest set on add; status dirty → clean after consumer observes.
Edge Before Producer Output, Then Output Appears
Steps: Add edge (no outputs) → producer posts outputs (serial 1).
Assert: status transitions pending→dirty with InDigest set; consumer observe makes it clean.
Mock Edges

Mock → Output Transition
Steps: Add edge with mock value → producer posts real output.
Assert: status mock→pending; MockValue cleared; InDigest set; later consumer observe makes it clean.
Mock Stays When Output Missing
Steps: Add mock edge → producer posts state without that key.
Assert: mock remains; no InDigest; later when key appears, transitions as above.
Output Lifecycle

Output Removal
Steps: Producer posts output (serial 1) → add edge → remove that output (serial 2).
Assert: status missing-output; InDigest unchanged or cleared per impl; remains missing until key reappears.
Output Rename
Steps: Producer switches from key A to key B between serials.
Assert: edge on A becomes missing-output; no silent “follow” to B.
Value Changes

Output Value Changes
Steps: Producer posts v1 (serial 1) → add edge → producer posts v2 (serial 2).
Assert: InDigest changes; status dirty until consumer observes again; OutDigest syncs to match.
Cache Usage

Cache Short‑Circuit on Edge Add
Steps: Producer writes tfstate (serial 1) without key X → add edge for X.
Assert: initializeEdgeIfProducerHasOutput uses cache to mark missing-output without parsing; status missing-output immediately.
Cache Present → Parse Only When Needed
Steps: Producer writes with key present → add edge.
Assert: guard detects key present; proceed to parse and set InDigest as now; verify no regression.
Sensitive Flag Changes

Sensitivity Flip Only Affects ListStateOutputs
Steps: Producer serial 1 with key K sensitive=false → serial 2 set sensitive=true.
Assert: ListStateOutputs reflects flag change; edge digest/status unaffected.
Multiple Edges/Keys

Multiple Edges From Same Producer
Steps: Two edges for different keys; change only one output.
Assert: Only corresponding edge’s InDigest/status changes; other edge remains clean.
Multiple Consumers Observing Same Producer
Steps: Two consumers depend on same producer key; producer updates once.
Assert: Both edges go dirty; each consumer observe cleans only its own edge (via incoming‑edges logic).
Locking/Transactions

Producer Locked, Update With Correct Lock ID
Steps: Lock producer → POST with ?ID=lock → confirm outputs cached and edge updated.
Assert: Cache and edge updates happen; status/digest consistent.
Serial‑Based Invalidation
Steps: Populate outputs at serial 1; then serial 2 with different keys.
Assert: state_outputs rows for serial 1 removed; only serial 2 used; ListStateOutputs matches serial 2.
Robustness/Flake‑proofing

Poll Instead of Sleep
Replace fixed sleeps with polling helper (retry until edge status matches or timeout) to reduce flakes from async edge updates.
