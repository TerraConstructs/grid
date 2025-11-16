import { Transport, createClient } from '@connectrpc/connect';
import type { DescService } from '@bufbuild/protobuf';
import { createConnectTransport } from '@connectrpc/connect-web';
import { StateService } from '../gen/state/v1/state_pb.js';
import type {
  ListStatesRequest,
  ListStatesResponse,
  GetStateInfoRequest,
  GetStateInfoResponse,
  ListAllEdgesRequest,
  ListAllEdgesResponse,
  GetLabelPolicyRequest,
  GetLabelPolicyResponse,
  SetLabelPolicyRequest,
  SetLabelPolicyResponse,
} from '../gen/state/v1/state_pb.js';

/**
 * Create a Connect transport from a base URL.
 *
 * @param baseUrl - Base URL of the Grid API server (e.g., 'http://localhost:8080')
 * @returns Connect transport for HTTP/2 communication
 *
 * ## Session Cookie Handling
 *
 * Session cookies (httpOnly) are sent automatically with all requests via the custom fetch override
 * which sets `credentials: 'include'`. This ensures cookies are sent for both same-origin and
 * cross-origin requests.
 *
 * **Fetch API Default**: The standard Fetch API default is `credentials: 'same-origin'`, which only
 * sends cookies for same-origin requests. By explicitly setting `credentials: 'include'`, we ensure
 * cookies are sent regardless of request origin.
 *
 * **Security Implications**:
 * - Same-origin requests: Cookies sent automatically (CSRF protection via SameSite attribute)
 * - Cross-origin requests: Requires server to send `Access-Control-Allow-Credentials: true` header
 * - Server must explicitly opt-in to cross-origin cookie requests
 *
 * @example
 * ```typescript
 * const transport = createGridTransport('http://localhost:8080');
 * const api = new GridApiAdapter(transport);
 * ```
 */
export function createGridTransport(baseUrl: string): Transport {
  return createConnectTransport({
    baseUrl,
    fetch: (input, init) => fetch(input, {...init, credentials: 'include'}),
  });
}

/**
 * Create a Grid API client from a transport.
 *
 * This is a low-level factory for creating Connect clients directly.
 * Most users should use GridApiAdapter instead.
 *
 * @param transport - Connect transport
 * @returns StateService client
 *
 * @example
 * ```typescript
 * const transport = createGridTransport('http://localhost:8080');
 * const client = createGridClient(transport);
 * const response = await client.listStates({});
 * ```
 */
export interface StateServiceClient {
  listStates(request?: ListStatesRequest | Record<string, unknown>): Promise<ListStatesResponse>;
  getStateInfo(
    request: GetStateInfoRequest | Record<string, unknown>
  ): Promise<GetStateInfoResponse>;
  listAllEdges(
    request?: ListAllEdgesRequest | Record<string, unknown>
  ): Promise<ListAllEdgesResponse>;
  getLabelPolicy(
    request?: GetLabelPolicyRequest | Record<string, unknown>
  ): Promise<GetLabelPolicyResponse>;
  setLabelPolicy(
    request: SetLabelPolicyRequest | Record<string, unknown>
  ): Promise<SetLabelPolicyResponse>;
}

export function createGridClient(transport: Transport): StateServiceClient {
  return createClient(
    StateService as unknown as DescService,
    transport
  ) as unknown as StateServiceClient;
}
