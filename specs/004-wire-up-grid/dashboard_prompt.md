# PoC Grid Dashboard

We have received a PoC Grid dashboard under ./webapp (React + Vite) using mock data see webapp/src/services/mockApi.ts

We have set up pnpm workspaces for the repository, so the WebApp can import from other packages like js/sdk.

The goal is to provide a demo of the Grid API in action asap (without AuthN/AuthZ or other unsupported API features) by replacing the mockApi with real API calls.

The webapp is only allowed to use what is defined in proto/state/v1/state.proto

We need to add a super-thin adapter layer between the WebApp and the generated clients in js/sdk to keep the WebApp decoupled from proto transport details and to allow for future other js/ts consumers (like our CDKTF library).

Review the state proto against the mockApi to identify the calls needed for the dashboard.
Consider patterns from the Go SDK (pkg/sdk) for inspiration.

- pkg/sdk/state_client.go
- pkg/sdk/state_types.go
- pkg/sdk/dependency.go

Example adapter interface.

```ts
// js/sdk (the adapter you control)
export interface GridClient {
  listStates(...): Promise<State[]>
  getState(...): Promise<State>
  // only what the dashboard needs (read-only)
}

// Implementation v1: wraps connect-web generated clients in /src/api/gen
export function makeGridClient(fetchCtx: FetchCtx): GridClient { ... }
```

* Keep the adapter interface tiny and ergonomic (your wrapper types).
* Put transport (baseURL, headers) behind `makeGridClient`.
* Use it everywhere in UI so your components never touch raw proto messages.

WebApp import from`@tcons/grid` using pnpm `workspace:*` protocol.

When AuthN/AuthZ arrives, implement it once in the sdk’s transport factory (browser: `@connectrpc/connect-web`; Node: `@connectrpc/connect-node` for your CDKTF lib). The WebApp just supplies a token getter.

---

# Extra considerations (to help choose)

* **Two TS consumers** (WebApp + CDKTF lib) argue for a central sdk; but your **immediate goal is read-only UI** → the adapter should focus on the needs of the webapp first.
* **Error UX**: normalize gRPC codes → human messages in the adapter now; keep the mapping when you move it into the sdk.
* **Bundle size**: generate only the services you need for the dashboard; ensure ESM + named exports so Vite tree-shakes.
* **Testing**: re-align mocks for the **adapter interface**, not the generated clients. Swap the adapter impl in tests.
* **Transports**: define one factory signature that works in browser and Node (headers, interceptors, tracing) so the sdk can share it later.
* **Keep all thin wrappers in `js/sdk` now**, export them as the public facade, and
* Have the WebApp (and later the CDKTF TS lib) consume `js/sdk` via the workspace protocol.

# Structure (minimal, tree-shakable)

Propose changes to js/sdk tree first, for example:

```
js/sdk/
  package.json              # "name": "@tcons/js-sdk"
  tsconfig.json
  src/
    client/
      transport.ts          # createConnectTransport(factory), auth header hook
      interceptors.ts       # error mapping, tracing/logging (optional)
    facade/
      grid.types.ts         # value-object types for UI (no protobufs)
      grid.mapper.ts        # proto -> VO mappers
      grid.client.ts        # thin wrapper (public API)
    index.ts                # re-exports only public facade types/functions
  gen/
    state/v1/state_connect.ts   # buf generate output (browser)
    state/v1/state_pb.ts        # messages
```

Key ideas:

* **All buf-generated code stays in `js/sdk/gen`**.
* **Only `src/facade/*` is public**. UI never touches proto messages.
* **`src/client/*`** holds transport/auth setup reusable by browser and Node.

# package.json (ESM + great DX)

Review existing `js/sdk/package.json` and adjust as needed, for example:

```json
{
  "name": "@tcons/js-sdk",
  "type": "module",
  "sideEffects": false,
  "exports": {
    ".": {
      "types": "./dist/index.d.ts",
      "import": "./dist/index.js"
    }
  }
}
```

# Transport factory (browser-first, Node-friendly later)

For example (validate against proto and existing pkg/sdk patterns):

```ts
// js/sdk/src/client/transport.ts
import { createConnectTransport } from "@connectrpc/connect-web";
import type { Interceptor } from "@connectrpc/connect";
import { authInterceptor, errorInterceptor } from "./interceptors";

export type TokenProvider = () => Promise<string | undefined> | string | undefined;

export function makeTransport(opts: {
  baseUrl: string;
  token?: TokenProvider;
  extraInterceptors?: Interceptor[];
}) {
  const interceptors: Interceptor[] = [
    errorInterceptor(),
    authInterceptor(opts.token),
    ...(opts.extraInterceptors ?? []),
  ];
  return createConnectTransport({ baseUrl: opts.baseUrl, interceptors });
}
```

No Auth yet, so this should be stubbed.

```ts
// js/sdk/src/client/interceptors.ts
import type { Interceptor, UnaryRequest, NextUnaryFn } from "@connectrpc/connect";

export const authInterceptor = (getToken?: () => string | Promise<string> | undefined): Interceptor =>
  (next: NextUnaryFn) => async (req: UnaryRequest) => {
    const token = getToken ? await getToken() : undefined;
    if (token) req.header.set("Authorization", `Bearer ${token}`);
    return next(req);
  };

export const errorInterceptor = (): Interceptor =>
  (next: NextUnaryFn) => async (req: UnaryRequest) => {
    try {
      return await next(req);
    } catch (e: any) {
      // Map Connect errors -> friendly SDKError
      throw normalizeError(e);
    }
  };

class SDKError extends Error { code?: string; details?: unknown; }
function normalizeError(e: any): SDKError {
  const err = new SDKError(e.message ?? "Request failed");
  err.code = e?.code;
  err.details = e?.rawMessage ?? e?.metadata;
  return err;
}
```

# Facade types + mappers (value objects, not protos)

```ts
// js/sdk/src/facade/grid.types.ts
export type StateId = string; // Grid GUID/logic-id as string

export interface StateSummary {
  id: StateId;
  name: string;
  updatedAt: string;
  workspace?: string;
}

export interface StateDetail extends StateSummary {
  outputs: Record<string, unknown>;
}
```

```ts
// js/sdk/src/facade/grid.mapper.ts
import type { State } from "../../gen/state/v1/state_pb"; // proto message
import type { StateDetail, StateSummary } from "./grid.types";

export function mapStateSummary(p: State): StateSummary {
  return {
    id: p.id,
    name: p.name,
    updatedAt: p.updatedAt, // ensure your proto has ISO string or convert Timestamp
    workspace: p.workspace || undefined,
  };
}

export function mapStateDetail(p: State): StateDetail {
  return {
    ...mapStateSummary(p),
    outputs: p.outputs ?? {},
  };
}
```

# Facade client (tiny, read-only to start)

```ts
// js/sdk/src/facade/grid.client.ts
import { makeTransport } from "../client/transport";
import { createPromiseClient } from "@connectrpc/connect";
import { StateService } from "../../gen/state/v1/state_connect";
import { mapStateDetail, mapStateSummary } from "./grid.mapper";
import type { StateDetail, StateSummary } from "./grid.types";

export interface GridClient {
  listStates(params?: { limit?: number; cursor?: string }): Promise<StateSummary[]>;
  getState(id: string): Promise<StateDetail>;
}

export function makeGridClient(opts: {
  baseUrl: string;
  token?: () => string | Promise<string> | undefined;
}): GridClient {
  const transport = makeTransport({ baseUrl: opts.baseUrl, token: opts.token });
  const svc = createPromiseClient(StateService, transport);

  return {
    async listStates(params) {
      const res = await svc.listStates({ pageSize: params?.limit, pageToken: params?.cursor });
      return (res.items ?? []).map(mapStateSummary);
    },
    async getState(id) {
      const res = await svc.getState({ id });
      return mapStateDetail(res.state!);
    },
  };
}
```

```ts
// js/sdk/src/index.ts
export * from "./facade/grid.client";
export * from "./facade/grid.types";
```

# WebApp usage (no mocks rewrite needed)

```ts
// webapp/src/lib/grid.ts
import { makeGridClient } from "@tcons/js-sdk";

export const grid = makeGridClient({
  baseUrl: import.meta.env.VITE_GRID_API_BASE_URL,
  token: () => localStorage.getItem("grid_token") ?? undefined, // temp; replace when AuthN lands
});
```

Now all React code consumes the **facade types** (`StateSummary`, `StateDetail`) and **never** touches proto messages. When AuthN/AuthZ arrives, you update `js/sdk/src/client/transport.ts` - the WebApp just supplies a token getter.

# Testing

* Unit test the mapper and facade with **proto fixtures** (from `gen`) or plain objects.
* Integration test with **MSW** at the transport boundary.
* In the WebApp, mock the **facade interface** (`GridClient`), not the generated clients.

# Pros of keeping the wrapper in `js/sdk` now

* Single source of truth for transport/auth/error mapping.
* Zero migration later - WebApp already imports from the right place.
* Unblocks the CDKTF TS library to reuse the same facade tomorrow.
* Matches your Go pattern (`pkg/sdk`), keeping DevX uniform across languages.
* With pnpm workspaces, there’s no publish friction for local development.

# Tradeoffs

* Slightly more initial work than dropping code straight into `webapp/src/api`, but you avoid the certain refactor later.
* You’ll want to keep `js/sdk`’s public surface tight (only what the dashboard needs) and grow it incrementally.

# Immediate steps (1–2 short sessions worth)

1. Confirm **buf generate** are wired to `js/sdk/gen` (browser target).
2. Implement the **facade** (`grid.types.ts`, `grid.mapper.ts`, `grid.client.ts`) for the 2–3 read-only calls your dashboard needs.
3. Expose `makeGridClient` from `js/sdk` and replace the WebApp mocks by injecting the client.
4. Keep auth **stubbed** (localStorage token) until AuthN lands.
