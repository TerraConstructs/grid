- [x] Bootstrap golang monorepo (gridctl -> gridapi works, client packages: pkg/sdk + js/sdk And integration testing)
- [x] Add basic Terraform State HTTP backend implementation
- [x] Add State Dependency management (track directed acyclic multigraph of states).
  - [x] Parse outputs from states, add queries to find outputs and define from_state,from_output -> to_state definitions.
  - [x] Track attributes on state edges ("Status" to track if output value has changed since "last observed" by dependent state. Define "last observed" in relation to TF State backend HTTP activity)
  - [x] Cycle detection and prevention
  - [ ] Ability to define "mock outputs" on a State to allow creating edges before state (and outputs..) exist in gridapi.
  - [x] Update State list to show status of dependencies (and if state is stale or potentially stale -> transitive dependency changed)
  - [x] Include HCL Generation for dependencies (generate terraform code to consume outputs from other states into blocks of "locals"). Add ability to "sync" local file on disk (flag block as "managed" to prevent user edits)
  - [X] Ability to get state info in CLI (including outputs, dependencies, dependents)
  - [X] gridctl should store .grid context in directory it runs, tracking state info. By default any command should look for .grid dir in current dir and use the state info from there (to make managing the state and its dependencies easier).
- [X] Add webapp (React + Vite), consume js/sdk - use static html/css/js. Allow serve from nginx or directly from gridapi binary (embedded files) on /dashboard path. No CRUD - read only for now
  - [X] CORS config for API and Webapp (basic localhost, deferred for later)
  - [X] Use Vite Dev for local development with hmr
  - [X] Graph view of states and dependencies and state of edges. render topo layers; color **dirty** edges; click to drill.
  - [X] List view of states, dependencies:  their states and their Typescript schemas
  - [X] Detail view of state (json view of tfstate, list of dependencies, list of dependents)
- [ ] Review TLS / GRPC set up
  - [ ] Does HTTP2 require TLS (what is h2c?)
  - [ ] How to set up local dev with TLS
- [ ] TerraConstructs (Typescript) ContextProvider implementation leveraging the Grid API
  - [ ] gridctl: Add ability to attach Typescript schema to state Outputs
  - [ ] gridctl: Add ability to generate Typescript declaration files for cross state dependencies
- [ ] Release pipelines for Alpha
  - [ ] Dockerfile for gridapi (publish to github container registry) 
  - [ ] binary releases for both gridapi and gridctl
  - [ ] js/sdk publish to npmjs as `@tcons/grid` (?)
  - [ ] release-please config and GH Workflows
- [ ] Implement AuthN/AuthZ
  - [ ] Chi OAuth Middleware?
  - [ ] RBAC (SpiceDB or in-memory?)
  - [ ] CLI config on disk with ability login/logout commands, store token in config file
  - [ ] TFE endpoints?
  - [ ] Audit logging (who did what, when)
- [ ] Add Observability (structured logging, prometheus metrics, pprof)
  - [ ] use uptrace/bun otel functionality for sql metrics
  - [ ] Add chi middleware for request logging, metrics, tracing
  - [ ] OTEL tracing to service layer (business logic telemetry, error events)
  - [ ] Include build info (version, commit, date) in binary and expose on /healthz endpoint

QA Feedback:

- [X] The State status isn't shown correctly in the UI
- [ ] How to get back the .grid if I accidentally delete it?
- [ ] Add polling to the dashboard
- [ ] The flags are inconsistent between commands (e.g. --state vs --logic-id vs --state-from vs positional args)

Nice to haves:

- [ ] SQLite support for local instance of gridapi (local file based sqlite db, no docker, just run binary and use local file for graph)
- [ ] Single machine serve mode (local sqlite db file provider for roll out without RDS dependency)
  - [ ] Add SQLite bun driver and repository implementations
  - [ ] Verify with integration tests
  - [ ] Add litestream support to replicate sqlite db to s3 for durability
  - [ ] Reference: [opentaco](https://github.com/diggerhq/digger/pull/2265/files#top)
- [ ] Add State metadata (kvp, with schema to control allowed keys and value lists), allow state queries based on metadata filtering, including in dependency queries
- [ ] Add state grouping (allow metadata inheritance from group to state, allow group level metadata and queries) - but not group based dependency management (yet?). Add nested groups and layer metadata across groups (Aggregate at state level)
- [ ] Add State statuses to track the Terraform/CDKTF process (pendingDependencies, readyForPlan, planned, policyApproved, applyApproved, applied, ...)
- [ ] Control RBAC based on metadata and/or state groups
- [ ] `gridctl state create --init` to create state and write out backend.tf in 1 go
- [ ] use directory name as default logic-id (similar to how `gh create --fill` uses git commit message as title and body)
- [ ] `gridapi serve --migrate` to run migrations init and up on startup
- [ ] Use AWS IAM as a Authn provider (similar to how kubeapi -> used sts signed username verification to confirm identity)
- [ ] Add state versioning (track changes to state over time, allow rollback to previous versions)
- [ ] Parse out TFState json (track resource, resource attributes, resource graph, etc), provide MCP to provide LLM capabilities to query state graph and create IaC based on state graph (e.g. create a k8s cluster would automatically identify the states owning the network and so forth).
- [ ] Gridctl functionality similar to tfmigrate (handle resource migrations between grid states)
- [ ] Rename modules to match `github.com/terraconstructs/grid/pkg/sdk`, `github.com/terraconstructs/grid/api`, ...
  - [ ] `github.com/terraconstructs/grid/cmd/gridapi` -> `github.com/terraconstructs/grid/cmd/server`(still builds bin/gridapi)
  - [ ] `github.com/terraconstructs/grid/cmd/gridctl` -> `github.com/terraconstructs/grid/cmd/cli` (still builds bin/gridctl)
- [ ] Add ability to store tfstate in encrypted format (at rest) - use envelope encryption with KMS (AWS KMS, GCP KMS, Azure Key Vault, HashiCorp Vault)

Review Spacelift, Env0, TerraMate, Terragrunt scale, Digger/OpenTaco interesting features for comparison and inspiration.


## UX improvements

### Flags are inconsistent between commands (e.g. --state vs --logic-id vs --state-from vs positional args)

Context: Users must remember different flag names and argument styles for similar concepts across commands, leading to confusion and errors.

Findings

  - gridctl top-level groups mix singular and abbreviated names (gridctl state vs gridctl deps), which makes the CLI surface inconsistent; see cmd/gridctl/cmd/state/state.go:16 and cmd/gridctl/cmd/deps/deps.go:16.
  - Flags for targeting a state jump between --logic-id, --state, and --to, so users must remember different spellings for the same concept; see cmd/gridctl/cmd/state/get.go:126, cmd/gridctl/cmd/deps/list.go:111,
    cmd/gridctl/cmd/deps/status.go:104, cmd/gridctl/cmd/deps/sync.go:202, cmd/gridctl/cmd/deps/add.go:124.
  - state subcommands mix positional arguments (create/init) with optional flags (get), so the way you select a state changes across sibling commands; see cmd/gridctl/cmd/state/create.go:20, cmd/gridctl/cmd/state/
    init.go:22, cmd/gridctl/cmd/state/get.go:126.
  - Help text oscillates between logic-id, logic ID, and logic_id, adding unnecessary cognitive overhead; see cmd/gridctl/cmd/state/create.go:22, cmd/gridctl/cmd/state/init.go:24, cmd/gridctl/cmd/state/get.go:126.

  Next steps:

  1. Pick a consistent noun form for the top-level command groups and align the Use strings.
  2. Standardize how callers provide state identifiers (flag name + positional behaviour + copy text) across state and deps, then update the long descriptions accordingly.

  ## WebApp

  - Finding 1 (perf): getEdgePathData repeatedly does findIndex over the parallel and outgoing groups for every render (webapp/src/components/GraphView.tsx:168-190). With many edges this degenerates to O(n²) cost per frame. You can precompute index/slot metadata when you build parallelGroups/outgoingGroups (e.g. alongside the Map, keep a Map<edgeId, {slotIndex,total}>) so each edge lookup becomes O(1).
  - Finding 2 (perf): The directionalGroup filter inside getEdgePathData re-walks a node’s entire outgoing set even when no tooltip/hover is active (webapp/src/components/GraphView.tsx:175-183). Consider partitioning the outgoing map up-front by (from_guid, direction) or memoizing the filtered lists to avoid repeated filtering.
  - Finding 3 (perf): For hovered edges getEdgePathData runs twice—once in renderEdge, once in renderTooltip (webapp/src/components/GraphView.tsx:205-237). Memoizing the geometry map (useMemo → Map<edgeId, Geometry>) would let both renderers reuse the same object and spare the duplicate math.

Open question: would precomputing a single edgeGeometry map (including tooltip positions) suffice for both renderers, or do we need the raw groups elsewhere?

Second review:

GraphView Edges

- Built directional edge buckets that sort by target node x-position so fan-out ordering follows the layout (webapp/src/components/GraphView.tsx:147-182).
- Added DirectedEdgeGroup helper and consume the sorted groups when computing anchor slots, keeping horizontal lanes evenly distributed even with mixed directions (webapp/src/components/GraphView.tsx:205-217).
