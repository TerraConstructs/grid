export interface BackendConfig {
  address: string;
  lock_address: string;
  unlock_address: string;
}

export interface StateRef {
  guid: string;
  logic_id: string;
}

export interface DependencyEdge {
  id: number;
  from_guid: string;
  from_logic_id: string;
  from_output: string;
  to_guid: string;
  to_logic_id: string;
  to_input_name?: string;
  status: 'pending' | 'clean' | 'dirty' | 'potentially-stale' | 'mock' | 'missing-output';
  in_digest?: string;
  out_digest?: string;
  mock_value_json?: string;
  last_in_at?: string;
  last_out_at?: string;
  created_at: string;
  updated_at: string;
}

export interface OutputKey {
  key: string;
  sensitive: boolean;
}

export interface StateInfo {
  guid: string;
  logic_id: string;
  locked: boolean;
  created_at: string;
  updated_at: string;
  size_bytes: number;
  computed_status?: string;
  dependency_logic_ids: string[];
  backend_config: BackendConfig;
  dependencies: DependencyEdge[];
  dependents: DependencyEdge[];
  outputs: OutputKey[];
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  state_json?: any;
}

const mockStates: StateInfo[] = [
  {
    guid: '01HZXK1234567890ABCDEF0001',
    logic_id: 'prod/network',
    locked: false,
    created_at: '2025-10-01T10:00:00Z',
    updated_at: '2025-10-01T14:30:00Z',
    size_bytes: 45320,
    computed_status: 'clean',
    dependency_logic_ids: [],
    backend_config: {
      address: 'https://api.example.com/states/01HZXK1234567890ABCDEF0001',
      lock_address: 'https://api.example.com/states/01HZXK1234567890ABCDEF0001/lock',
      unlock_address: 'https://api.example.com/states/01HZXK1234567890ABCDEF0001/unlock',
    },
    dependencies: [],
    dependents: [],
    outputs: [
      { key: 'vpc_id', sensitive: false },
      { key: 'app_subnets', sensitive: false },
      { key: 'data_subnets', sensitive: false },
      { key: 'ingress_nlb_sg', sensitive: false },
      { key: 'ingress_nlb_id', sensitive: false },
      { key: 'domain_zone_id', sensitive: false },
    ],
    state_json: {
      vpc_id: 'vpc-0a1b2c3d4e5f6',
      app_subnets: ['subnet-app-1a', 'subnet-app-1b', 'subnet-app-1c'],
      data_subnets: ['subnet-data-1a', 'subnet-data-1b', 'subnet-data-1c'],
      ingress_nlb_sg: 'sg-nlb-ingress-001',
      ingress_nlb_id: 'nlb-prod-ingress',
      domain_zone_id: 'Z1234567890ABC',
    }
  },
  {
    guid: '01HZXK1234567890ABCDEF0002',
    logic_id: 'prod/cluster',
    locked: false,
    created_at: '2025-10-01T10:15:00Z',
    updated_at: '2025-10-01T14:00:00Z',
    size_bytes: 78540,
    computed_status: 'stale',
    dependency_logic_ids: ['prod/network'],
    backend_config: {
      address: 'https://api.example.com/states/01HZXK1234567890ABCDEF0002',
      lock_address: 'https://api.example.com/states/01HZXK1234567890ABCDEF0002/lock',
      unlock_address: 'https://api.example.com/states/01HZXK1234567890ABCDEF0002/unlock',
    },
    dependencies: [],
    dependents: [],
    outputs: [
      { key: 'cluster_node_sg', sensitive: false },
      { key: 'cluster_endpoint', sensitive: false },
      { key: 'cluster_ca_cert', sensitive: true },
    ],
    state_json: {
      cluster_node_sg: 'sg-cluster-nodes-001',
      cluster_endpoint: 'https://k8s-prod.example.com',
      cluster_ca_cert: '-----BEGIN CERTIFICATE-----\n...',
    }
  },
  {
    guid: '01HZXK1234567890ABCDEF0003',
    logic_id: 'prod/db',
    locked: false,
    created_at: '2025-10-01T10:30:00Z',
    updated_at: '2025-10-01T15:00:00Z',
    size_bytes: 32100,
    computed_status: 'clean',
    dependency_logic_ids: ['prod/network'],
    backend_config: {
      address: 'https://api.example.com/states/01HZXK1234567890ABCDEF0003',
      lock_address: 'https://api.example.com/states/01HZXK1234567890ABCDEF0003/lock',
      unlock_address: 'https://api.example.com/states/01HZXK1234567890ABCDEF0003/unlock',
    },
    dependencies: [],
    dependents: [],
    outputs: [
      { key: 'db_sg_id', sensitive: false },
      { key: 'db_endpoint', sensitive: false },
      { key: 'db_password', sensitive: true },
    ],
    state_json: {
      db_sg_id: 'sg-database-001',
      db_endpoint: 'prod-db.c9xyz.us-east-1.rds.amazonaws.com',
      db_password: '***REDACTED***',
    }
  },
  {
    guid: '01HZXK1234567890ABCDEF0004',
    logic_id: 'prod/app1_deploy',
    locked: false,
    created_at: '2025-10-01T11:00:00Z',
    updated_at: '2025-10-01T13:45:00Z',
    size_bytes: 56780,
    computed_status: 'stale',
    dependency_logic_ids: ['prod/network', 'prod/cluster', 'prod/db'],
    backend_config: {
      address: 'https://api.example.com/states/01HZXK1234567890ABCDEF0004',
      lock_address: 'https://api.example.com/states/01HZXK1234567890ABCDEF0004/lock',
      unlock_address: 'https://api.example.com/states/01HZXK1234567890ABCDEF0004/unlock',
    },
    dependencies: [],
    dependents: [],
    outputs: [
      { key: 'app_url', sensitive: false },
      { key: 'deployment_id', sensitive: false },
    ],
    state_json: {
      app_url: 'https://app1.example.com',
      deployment_id: 'deploy-app1-20251001-001',
    }
  },
];

const mockEdges: DependencyEdge[] = [
  {
    id: 1,
    from_guid: '01HZXK1234567890ABCDEF0001',
    from_logic_id: 'prod/network',
    from_output: 'vpc_id',
    to_guid: '01HZXK1234567890ABCDEF0002',
    to_logic_id: 'prod/cluster',
    to_input_name: 'vpc_id',
    status: 'dirty',
    in_digest: 'sha256:abc123',
    out_digest: 'sha256:def456',
    created_at: '2025-10-01T10:15:00Z',
    updated_at: '2025-10-01T14:30:00Z',
  },
  {
    id: 2,
    from_guid: '01HZXK1234567890ABCDEF0001',
    from_logic_id: 'prod/network',
    from_output: 'app_subnets',
    to_guid: '01HZXK1234567890ABCDEF0002',
    to_logic_id: 'prod/cluster',
    to_input_name: 'app_subnets',
    status: 'dirty',
    in_digest: 'sha256:ghi789',
    out_digest: 'sha256:jkl012',
    created_at: '2025-10-01T10:15:00Z',
    updated_at: '2025-10-01T14:30:00Z',
  },
  {
    id: 3,
    from_guid: '01HZXK1234567890ABCDEF0001',
    from_logic_id: 'prod/network',
    from_output: 'ingress_nlb_sg',
    to_guid: '01HZXK1234567890ABCDEF0002',
    to_logic_id: 'prod/cluster',
    to_input_name: 'ingress_nlb_sg',
    status: 'dirty',
    in_digest: 'sha256:mno345',
    out_digest: 'sha256:pqr678',
    created_at: '2025-10-01T10:15:00Z',
    updated_at: '2025-10-01T14:30:00Z',
  },
  {
    id: 4,
    from_guid: '01HZXK1234567890ABCDEF0001',
    from_logic_id: 'prod/network',
    from_output: 'data_subnets',
    to_guid: '01HZXK1234567890ABCDEF0003',
    to_logic_id: 'prod/db',
    to_input_name: 'data_subnets',
    status: 'clean',
    in_digest: 'sha256:stu901',
    out_digest: 'sha256:stu901',
    created_at: '2025-10-01T10:30:00Z',
    updated_at: '2025-10-01T15:00:00Z',
  },
  {
    id: 5,
    from_guid: '01HZXK1234567890ABCDEF0003',
    from_logic_id: 'prod/db',
    from_output: 'db_sg_id',
    to_guid: '01HZXK1234567890ABCDEF0004',
    to_logic_id: 'prod/app1_deploy',
    to_input_name: 'db_sg_id',
    status: 'clean',
    in_digest: 'sha256:vwx234',
    out_digest: 'sha256:vwx234',
    created_at: '2025-10-01T11:00:00Z',
    updated_at: '2025-10-01T13:45:00Z',
  },
  {
    id: 6,
    from_guid: '01HZXK1234567890ABCDEF0002',
    from_logic_id: 'prod/cluster',
    from_output: 'cluster_node_sg',
    to_guid: '01HZXK1234567890ABCDEF0004',
    to_logic_id: 'prod/app1_deploy',
    to_input_name: 'cluster_node_sg',
    status: 'dirty',
    in_digest: 'sha256:yza567',
    out_digest: 'sha256:bcd890',
    created_at: '2025-10-01T11:00:00Z',
    updated_at: '2025-10-01T14:00:00Z',
  },
  {
    id: 7,
    from_guid: '01HZXK1234567890ABCDEF0001',
    from_logic_id: 'prod/network',
    from_output: 'domain_zone_id',
    to_guid: '01HZXK1234567890ABCDEF0004',
    to_logic_id: 'prod/app1_deploy',
    to_input_name: 'domain_zone_id',
    status: 'pending',
    created_at: '2025-10-01T11:00:00Z',
    updated_at: '2025-10-01T11:00:00Z',
  },
  {
    id: 8,
    from_guid: '01HZXK1234567890ABCDEF0001',
    from_logic_id: 'prod/network',
    from_output: 'ingress_nlb_id',
    to_guid: '01HZXK1234567890ABCDEF0004',
    to_logic_id: 'prod/app1_deploy',
    to_input_name: 'ingress_nlb_id',
    status: 'pending',
    created_at: '2025-10-01T11:00:00Z',
    updated_at: '2025-10-01T11:00:00Z',
  },
];

mockStates.forEach(state => {
  state.dependencies = mockEdges.filter(e => e.to_guid === state.guid);
  state.dependents = mockEdges.filter(e => e.from_guid === state.guid);
});

export const mockApi = {
  listStates: async (): Promise<StateInfo[]> => {
    await new Promise(resolve => setTimeout(resolve, 300));
    return mockStates;
  },

  getStateInfo: async (logicId: string): Promise<StateInfo | null> => {
    await new Promise(resolve => setTimeout(resolve, 200));
    return mockStates.find(s => s.logic_id === logicId) || null;
  },

  listDependencies: async (logicId: string): Promise<DependencyEdge[]> => {
    await new Promise(resolve => setTimeout(resolve, 150));
    const state = mockStates.find(s => s.logic_id === logicId);
    return state?.dependencies || [];
  },

  listDependents: async (logicId: string): Promise<DependencyEdge[]> => {
    await new Promise(resolve => setTimeout(resolve, 150));
    const state = mockStates.find(s => s.logic_id === logicId);
    return state?.dependents || [];
  },

  getAllEdges: async (): Promise<DependencyEdge[]> => {
    await new Promise(resolve => setTimeout(resolve, 200));
    return mockEdges;
  },
};
