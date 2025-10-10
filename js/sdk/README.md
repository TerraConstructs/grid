# @tcons/grid - TypeScript SDK

TypeScript SDK for interacting with the Grid Terraform state management API.

## Installation

```bash
npm install @tcons/grid
```

## Quick Start

```typescript
import { GridClient } from '@tcons/grid';

const client = new GridClient({
  baseUrl: 'http://localhost:8080'
});

// List states with labels
const states = await client.listStates({
  includeLabels: true
});

console.log(states);
```

## API Reference

### GridClient

#### Constructor

```typescript
new GridClient(options: GridClientOptions)
```

**Options:**
- `baseUrl`: Grid API server URL (default: `http://localhost:8080`)

#### Methods

##### `listStates(options?)`

List all states with optional filtering and label inclusion.

```typescript
const states = await client.listStates({
  filter: 'env == "prod"',
  includeLabels: true
});
```

**Options:**
- `filter?: string` - Bexpr filter expression
- `includeLabels?: boolean` - Include labels in response (default: true)

**Returns:** `Promise<StateSummary[]>`

##### `getState(logicId)`

Get detailed information about a specific state.

```typescript
const state = await client.getState('my-state');
```

**Returns:** `Promise<StateInfo>`

##### `createState(logicId, options?)`

Create a new state with optional labels.

```typescript
await client.createState('my-state', {
  labels: {
    env: 'prod',
    team: 'platform'
  }
});
```

**Options:**
- `labels?: Record<string, string | number | boolean>` - Initial labels

**Returns:** `Promise<CreateStateResponse>`

##### `updateLabels(stateId, adds?, removals?)`

Update labels on an existing state.

```typescript
await client.updateLabels(
  'state-guid-here',
  { env: 'staging', region: 'us-west' },  // adds
  ['old-label']  // removals
);
```

**Parameters:**
- `stateId: string` - State GUID
- `adds?: Record<string, string | number | boolean>` - Labels to add/update
- `removals?: string[]` - Label keys to remove

**Returns:** `Promise<UpdateLabelsResponse>`

##### `getLabelPolicy()`

Retrieve the current label validation policy.

```typescript
const policy = await client.getLabelPolicy();
console.log(policy.policyJson);
```

**Returns:** `Promise<LabelPolicy>`

##### `setLabelPolicy(policyJson)`

Update the label validation policy.

```typescript
await client.setLabelPolicy({
  allowed_keys: { env: {}, team: {} },
  allowed_values: {
    env: ['prod', 'staging']
  },
  max_keys: 32,
  max_value_len: 256
});
```

**Returns:** `Promise<LabelPolicy>`

## Label Operations

### Label Format

Labels are key-value pairs with the following constraints:

- **Keys**: Must start with lowercase letter, contain only lowercase alphanumeric, underscore, or forward-slash, ≤32 characters
- **Values**: Can be string (≤256 chars), number, or boolean

### Label Policy

The label policy defines validation rules:

```json
{
  "allowed_keys": {
    "env": {},
    "team": {},
    "region": {}
  },
  "allowed_values": {
    "env": ["prod", "staging", "dev"],
    "region": ["us-west", "us-east"]
  },
  "reserved_prefixes": ["grid.io/"],
  "max_keys": 32,
  "max_value_len": 256
}
```

## Bexpr Filtering

The SDK provides utilities for building bexpr filter expressions used to query states by labels.

### Filter Utilities

```typescript
import { buildEqualityFilter, buildInFilter, combineFilters } from '@tcons/grid/filters/bexpr';

// Simple equality
const filter = buildEqualityFilter('env', 'prod');
// Result: 'env == "prod"'

// Multiple values (IN operator)
const filter = buildInFilter('env', ['prod', 'staging']);
// Result: 'env in ["prod","staging"]'

// Combine multiple conditions
const filter = combineFilters(
  [
    buildEqualityFilter('env', 'prod'),
    buildEqualityFilter('team', 'platform')
  ],
  'and'
);
// Result: 'env == "prod" and team == "platform"'
```

### Bexpr Syntax

Bexpr supports rich boolean expressions:

**Operators:**
- Equality: `==`, `!=`
- Comparison: `<`, `<=`, `>`, `>=`
- Membership: `in`, `not in`
- Matching: `matches` (regex)
- Boolean: `and`, `or`, `not`

**Examples:**

```typescript
// Simple equality
filter: 'env == "prod"'

// IN operator
filter: 'env in ["prod", "staging"]'

// Complex conditions with AND/OR
filter: '(env == "prod" or env == "staging") and team == "platform"'

// Numeric comparison
filter: 'generation >= 2'

// Boolean values
filter: 'active == true'

// Regex matching
filter: 'region matches "us-.*"'
```

**Important:**
- String values must be quoted
- Numeric and boolean values are unquoted
- Use parentheses for complex expressions

### Filter Helpers

#### `buildEqualityFilter(key, value)`

Create a simple equality filter.

```typescript
buildEqualityFilter('env', 'prod')
// Returns: 'env == "prod"'

buildEqualityFilter('active', true)
// Returns: 'active == true'
```

#### `buildInFilter(key, values)`

Create an IN filter for multiple allowed values.

```typescript
buildInFilter('env', ['prod', 'staging'])
// Returns: 'env in ["prod","staging"]'
```

#### `combineFilters(filters, operator)`

Combine multiple filters with AND or OR.

```typescript
combineFilters(
  [
    buildEqualityFilter('env', 'prod'),
    buildEqualityFilter('region', 'us-west')
  ],
  'and'
)
// Returns: 'env == "prod" and region == "us-west"'

combineFilters(
  [
    buildInFilter('env', ['prod', 'staging']),
    buildEqualityFilter('team', 'core')
  ],
  'or'
)
// Returns: 'env in ["prod","staging"] or team == "core"'
```

## Types

### StateSummary

```typescript
interface StateSummary {
  guid: string;
  logicId: string;
  locked: boolean;
  sizeBytes: number;
  createdAt: Date;
  updatedAt: Date;
  labels?: Record<string, string | number | boolean>;
}
```

### StateInfo

```typescript
interface StateInfo {
  guid: string;
  logicId: string;
  backendConfig: BackendConfig;
  dependencies: Edge[];
  dependents: Edge[];
  outputs: OutputKey[];
  createdAt: Date;
  updatedAt: Date;
}
```

### LabelPolicy

```typescript
interface LabelPolicy {
  version: number;
  policyJson: string; // JSON string of PolicyDefinition
  createdAt: Date;
  updatedAt: Date;
}
```

## Examples

### Filter states by environment

```typescript
import { buildEqualityFilter } from '@tcons/grid/filters/bexpr';

const filter = buildEqualityFilter('env', 'prod');
const prodStates = await client.listStates({ filter });
```

### Update labels with validation

```typescript
try {
  await client.updateLabels(
    stateGuid,
    { env: 'prod', team: 'platform' }
  );
} catch (error) {
  // Handle validation error
  console.error('Label validation failed:', error);
}
```

### Set policy with enum constraints

```typescript
await client.setLabelPolicy({
  allowed_keys: {
    env: {},
    team: {},
    region: {}
  },
  allowed_values: {
    env: ['prod', 'staging'],
    region: ['us-west', 'us-east', 'eu-west-1']
  },
  max_keys: 32,
  max_value_len: 256
});
```

### Extract enums from policy for UI dropdowns

```typescript
const policy = await client.getLabelPolicy();
const policyDef = JSON.parse(policy.policyJson);

// Get allowed values for a specific key
const envValues = policyDef.allowed_values?.env || [];
// Use in dropdown: ['prod', 'staging']
```

## License

MIT
