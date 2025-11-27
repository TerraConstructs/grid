# Webapp Design: Output Schema & Validation Display

## Overview

This document describes the UI/UX changes required in the Grid webapp to display:

1. **Output Schema metadata** on state detail modal
2. **Validation status** for outputs with schemas
3. **New edge status** (`schema-invalid`) across all views

The webapp is **read-only** - no UI elements for creating/updating schemas are needed at this stage.

## Current State Analysis

### Component Architecture

```
App.tsx
  ├─> GraphView.tsx (Dependency graph visualization)
  │    ├─> GridNode.tsx (State nodes)
  │    └─> GridEdge.tsx (Dependency edges with status colors)
  ├─> ListView.tsx (Table view of states and edges)
  └─> DetailView.tsx (State details modal)
       └─> Tabs: Overview | Labels | Dependencies | Dependents
```

### Current Data Models

**TypeScript Interface:** `js/sdk/src/models/state-info.ts`

```typescript
export interface OutputKey {
  key: string;
  sensitive: boolean;
}

export type EdgeStatus =
  | 'pending'
  | 'clean'
  | 'dirty'
  | 'potentially-stale'
  | 'mock'
  | 'missing-output';
```

### Current Output Display

**Location:** `DetailView.tsx` lines 129-146 (Overview tab)

```tsx
<div>
  <h3 className="text-xs font-medium text-gray-500 mb-2">
    Outputs ({state.outputs.length})
  </h3>
  <div className="grid grid-cols-2 gap-2">
    {state.outputs.map((output) => (
      <div key={output.key} className="flex items-center justify-between bg-gray-50 rounded-lg px-3 py-2">
        <code className="text-sm text-purple-600">{output.key}</code>
        {output.sensitive && (
          <span className="text-xs bg-red-100 text-red-700 px-2 py-0.5 rounded">
            sensitive
          </span>
        )}
      </div>
    ))}
  </div>
</div>
```

**Issues:**
- ❌ Simple 2-column grid - no space for schema metadata
- ❌ No validation status indicators
- ❌ No expandable details for schema viewing
- ✅ Already shows sensitive flag (pattern to follow)

## Design Changes

### 1. TypeScript Model Updates

#### 1.1 Extend OutputKey Interface

**File:** `js/sdk/src/models/state-info.ts`

```typescript
export interface OutputKey {
  /** Output name from Terraform state */
  key: string;

  /** Whether output is marked sensitive in Terraform */
  sensitive: boolean;

  /** JSON Schema definition (optional) */
  schema_json?: string;

  /** Validation status: 'valid', 'invalid', 'error', or 'not_validated' */
  validation_status?: 'valid' | 'invalid' | 'error' | 'not_validated';

  /** Validation error message (only present if validation_status is 'invalid' or 'error') */
  validation_error?: string;

  /** Last validation timestamp (ISO 8601) */
  validated_at?: string;
}
```

#### 1.2 Extend EdgeStatus Type

**File:** `js/sdk/src/models/state-info.ts`

```typescript
export type EdgeStatus =
  | 'pending'           // Edge created, no digest values yet
  | 'clean'             // in_digest === out_digest AND valid (synchronized AND valid)
  | 'clean-invalid'     // NEW in_digest === out_digest AND invalid (synchronized but fails schema)
  | 'dirty'             // in_digest !== out_digest AND valid (out of sync but valid)
  | 'dirty-invalid'     // NEW in_digest !== out_digest AND invalid (out of sync AND fails schema)
  | 'potentially-stale' // Producer updated, consumer not re-evaluated
  | 'mock'              // Using mock_value_json
  | 'missing-output';   // Producer doesn't have required output
```

#### 1.3 Update Protobuf Adapter

**File:** `js/sdk/src/adapter.ts`

Update `convertProtoOutputKey()` function:

```typescript
function convertProtoOutputKey(output: ProtoOutputKey): OutputKey {
  return {
    key: output.key,
    sensitive: output.sensitive,
    schema_json: output.schemaJson || undefined,
    validation_status: output.validationStatus
      ? (output.validationStatus as OutputKey['validation_status'])
      : undefined,
    validation_error: output.validationError || undefined,
    validated_at: output.validatedAt ? timestampToISO(output.validatedAt) : undefined,
  };
}
```

### 2. DetailView Component Updates

#### 2.1 New "Outputs" Tab

**Rationale:**
- Outputs with schemas require more vertical space (schema preview, validation status)
- Moving outputs to dedicated tab prevents Overview tab from becoming cluttered
- Follows existing tab pattern (Overview, Labels, Dependencies, Dependents)

**File:** `webapp/src/components/DetailView.tsx`

**Changes:**

1. Add "Outputs" tab to navigation (line 46-64)
2. Remove outputs section from Overview tab (lines 129-146)
3. Create new Outputs tab content with enhanced output cards

#### 2.2 Enhanced Output Cards Design

**Visual Mockup:**

```
┌─────────────────────────────────────────────────────────────┐
│ Outputs (3)                                                 │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ ┌─────────────────────────────────────────────────────┐   │
│ │ vpc_id                              [sensitive]      │   │
│ │ ──────────────────────────────────────────────────   │   │
│ │ ✓ Schema: string, pattern: ^vpc-[a-z0-9]+$          │   │
│ │ ✓ Validated: 2 minutes ago                           │   │
│ │                                                       │   │
│ │ [View Schema] ▼                                      │   │
│ └─────────────────────────────────────────────────────┘   │
│                                                             │
│ ┌─────────────────────────────────────────────────────┐   │
│ │ subnet_ids                                           │   │
│ │ ──────────────────────────────────────────────────   │   │
│ │ ⚠ Schema Mismatch                                    │   │
│ │   Expected: array of strings matching ^subnet-       │   │
│ │   Error: value at index 0 does not match pattern    │   │
│ │ ⚠ Validated: 5 minutes ago                           │   │
│ │                                                       │   │
│ │ [View Schema] ▼                                      │   │
│ └─────────────────────────────────────────────────────┘   │
│                                                             │
│ ┌─────────────────────────────────────────────────────┐   │
│ │ availability_zones                                   │   │
│ │ ──────────────────────────────────────────────────   │   │
│ │ No schema defined                                    │   │
│ └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Component Code:**

```tsx
{activeTab === 'outputs' && (
  <div className="space-y-3">
    {state.outputs.length === 0 ? (
      <p className="text-gray-500 text-center py-8">No outputs defined</p>
    ) : (
      state.outputs.map((output) => (
        <OutputCard key={output.key} output={output} />
      ))
    )}
  </div>
)}
```

#### 2.3 OutputCard Component

**File:** `webapp/src/components/OutputCard.tsx` (NEW)

```tsx
import { FileJson, CheckCircle2, AlertTriangle, XCircle, ChevronDown, ChevronUp } from 'lucide-react';
import { useState } from 'react';
import type { OutputKey } from '@tcons/grid';

interface OutputCardProps {
  output: OutputKey;
}

export function OutputCard({ output }: OutputCardProps) {
  const [isSchemaExpanded, setIsSchemaExpanded] = useState(false);

  const hasSchema = !!output.schema_json;
  const validationStatus = output.validation_status;

  // Determine card styling based on validation status
  const getBorderColor = () => {
    if (!hasSchema) return 'border-gray-200';

    switch (validationStatus) {
      case 'valid':
        return 'border-green-200 bg-green-50/30';
      case 'invalid':
        return 'border-orange-200 bg-orange-50/30';
      case 'error':
        return 'border-red-200 bg-red-50/30';
      default:
        return 'border-gray-200';
    }
  };

  const getValidationIcon = () => {
    switch (validationStatus) {
      case 'valid':
        return <CheckCircle2 className="w-4 h-4 text-green-600" />;
      case 'invalid':
        return <AlertTriangle className="w-4 h-4 text-orange-600" />;
      case 'error':
        return <XCircle className="w-4 h-4 text-red-600" />;
      default:
        return null;
    }
  };

  const getValidationMessage = () => {
    if (!hasSchema) return null;

    switch (validationStatus) {
      case 'valid':
        return (
          <div className="flex items-center gap-2 text-sm text-green-700">
            <CheckCircle2 className="w-4 h-4" />
            <span>Schema validation passed</span>
          </div>
        );
      case 'invalid':
        return (
          <div className="space-y-1">
            <div className="flex items-center gap-2 text-sm text-orange-700 font-medium">
              <AlertTriangle className="w-4 h-4" />
              <span>Schema validation failed</span>
            </div>
            {output.validation_error && (
              <div className="ml-6 text-xs text-orange-600 bg-white rounded px-2 py-1 font-mono">
                {output.validation_error}
              </div>
            )}
          </div>
        );
      case 'error':
        return (
          <div className="space-y-1">
            <div className="flex items-center gap-2 text-sm text-red-700 font-medium">
              <XCircle className="w-4 h-4" />
              <span>Validation error</span>
            </div>
            {output.validation_error && (
              <div className="ml-6 text-xs text-red-600 bg-white rounded px-2 py-1 font-mono">
                {output.validation_error}
              </div>
            )}
          </div>
        );
      default:
        return null;
    }
  };

  const formatRelativeTime = (isoDate: string) => {
    const date = new Date(isoDate);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);

    if (diffMins < 1) return 'just now';
    if (diffMins < 60) return `${diffMins} minute${diffMins > 1 ? 's' : ''} ago`;

    const diffHours = Math.floor(diffMins / 60);
    if (diffHours < 24) return `${diffHours} hour${diffHours > 1 ? 's' : ''} ago`;

    const diffDays = Math.floor(diffHours / 24);
    return `${diffDays} day${diffDays > 1 ? 's' : ''} ago`;
  };

  const parseSchemaPreview = (schemaJson: string): string => {
    try {
      const schema = JSON.parse(schemaJson);

      // Build human-readable preview
      const parts: string[] = [];

      if (schema.type) {
        parts.push(`type: ${schema.type}`);
      }

      if (schema.pattern) {
        parts.push(`pattern: ${schema.pattern}`);
      }

      if (schema.minLength !== undefined || schema.maxLength !== undefined) {
        parts.push(`length: ${schema.minLength || 0}-${schema.maxLength || '∞'}`);
      }

      if (schema.enum) {
        parts.push(`enum: [${schema.enum.slice(0, 3).join(', ')}${schema.enum.length > 3 ? '...' : ''}]`);
      }

      if (schema.items?.type) {
        parts.push(`items: ${schema.items.type}`);
      }

      return parts.join(', ') || 'See JSON Schema below';
    } catch {
      return 'Invalid JSON Schema';
    }
  };

  return (
    <div className={`border rounded-lg transition-colors ${getBorderColor()}`}>
      <div className="p-3 space-y-2">
        {/* Header */}
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-2">
            <code className="text-base font-semibold text-purple-700">
              {output.key}
            </code>
            {output.sensitive && (
              <span className="inline-flex items-center text-xs bg-red-100 text-red-700 px-2 py-0.5 rounded font-medium">
                sensitive
              </span>
            )}
          </div>
          {getValidationIcon()}
        </div>

        {/* Schema Preview */}
        {hasSchema && (
          <div className="space-y-2">
            <div className="flex items-center gap-2 text-xs text-gray-600">
              <FileJson className="w-3.5 h-3.5 text-purple-500" />
              <span className="font-mono">{parseSchemaPreview(output.schema_json!)}</span>
            </div>

            {/* Validation Status */}
            {getValidationMessage()}

            {/* Validated Timestamp */}
            {output.validated_at && (
              <div className="text-xs text-gray-500">
                Validated {formatRelativeTime(output.validated_at)}
              </div>
            )}

            {/* Expandable Schema Viewer */}
            <button
              onClick={() => setIsSchemaExpanded(!isSchemaExpanded)}
              className="flex items-center gap-1.5 text-sm text-purple-600 hover:text-purple-700 font-medium transition-colors"
            >
              {isSchemaExpanded ? (
                <>
                  <ChevronUp className="w-4 h-4" />
                  Hide Schema
                </>
              ) : (
                <>
                  <ChevronDown className="w-4 h-4" />
                  View Schema
                </>
              )}
            </button>

            {/* Expanded Schema JSON */}
            {isSchemaExpanded && (
              <div className="mt-2 bg-gray-900 rounded-md p-3 overflow-auto max-h-60">
                <pre className="text-xs text-green-400 font-mono">
                  {JSON.stringify(JSON.parse(output.schema_json!), null, 2)}
                </pre>
              </div>
            )}
          </div>
        )}

        {/* No Schema Message */}
        {!hasSchema && (
          <div className="text-sm text-gray-500 italic">
            No schema defined
          </div>
        )}
      </div>
    </div>
  );
}
```

**Key Features:**
- ✅ **Visual hierarchy** - Output name prominent, schema details secondary
- ✅ **Validation indicators** - Color-coded borders and icons
- ✅ **Schema preview** - Human-readable summary extracted from JSON Schema
- ✅ **Error messages** - Full validation error displayed inline
- ✅ **Expandable viewer** - Click to show full JSON Schema
- ✅ **Sensitive flag** - Maintains existing UX pattern
- ✅ **Timestamps** - Relative time formatting (e.g., "2 minutes ago")

### 3. Edge Status Display Updates

#### 3.1 Graph View (GridEdge.tsx)

**File:** `webapp/src/components/graphflow/utils.ts`

Update `getEdgeColor()` function:

```typescript
export function getEdgeColor(status: string): string {
  switch (status) {
    case 'clean':
      return '#10b981'; // green-500
    case 'dirty':
      return '#f59e0b'; // amber-500
    case 'pending':
      return '#3b82f6'; // blue-500
    case 'potentially-stale':
      return '#f59e0b'; // amber-500
    case 'schema-invalid':  // NEW
      return '#ef4444'; // red-500 - Most severe status
    case 'missing-output':
      return '#dc2626'; // red-600
    default:
      return '#6b7280'; // gray-500
  }
}
```

**Visual Impact:**
- Schema-invalid edges will render in **red** (#ef4444)
- Color is more severe than dirty/stale (amber), less severe than missing-output (darker red)
- Hover tooltip already shows status text - no changes needed

#### 3.2 List View (ListView.tsx)

**File:** `webapp/src/components/ListView.tsx`

Update `getEdgeStatusBadge()` function (lines 27-42):

```typescript
const getEdgeStatusBadge = (status: string) => {
  const styles: Record<string, string> = {
    clean: 'bg-green-100 text-green-800',
    dirty: 'bg-orange-100 text-orange-800',
    pending: 'bg-blue-100 text-blue-800',
    'potentially-stale': 'bg-yellow-100 text-yellow-800',
    mock: 'bg-purple-100 text-purple-800',
    'missing-output': 'bg-red-100 text-red-800',
    'schema-invalid': 'bg-red-100 text-red-800 font-bold',  // NEW - Bold to stand out
  };

  return (
    <span className={`px-2 py-1 rounded text-xs font-medium ${styles[status] || 'bg-gray-100 text-gray-800'}`}>
      {status}
    </span>
  );
};
```

**Visual Impact:**
- Schema-invalid badge styled same as missing-output (red background)
- Added `font-bold` to make it more prominent
- Text "schema-invalid" displays in table cell

#### 3.3 Detail View Edges (DetailView.tsx)

**File:** `webapp/src/components/DetailView.tsx`

Update `getEdgeStatusColor()` function (lines 12-20):

```typescript
const getEdgeStatusColor = (status: string): string => {
  const colors: Record<string, string> = {
    clean: 'text-green-600 bg-green-50 border-green-200',
    dirty: 'text-orange-600 bg-orange-50 border-orange-200',
    pending: 'text-blue-600 bg-blue-50 border-blue-200',
    'potentially-stale': 'text-yellow-600 bg-yellow-50 border-yellow-200',
    'schema-invalid': 'text-red-600 bg-red-50 border-red-200',  // NEW
    'missing-output': 'text-red-700 bg-red-100 border-red-300',
  };
  return colors[status] || 'text-gray-600 bg-gray-50 border-gray-200';
};
```

**Visual Impact:**
- Dependency/dependent edge cards with schema-invalid status get red styling
- Slightly less saturated than missing-output to maintain visual hierarchy

### 4. Edge Status Semantics

**Status Priority (Highest to Lowest):**

```
1. missing-output   - Producer lacks required output (CRITICAL)
2. schema-invalid   - Output exists but fails validation (ERROR)
3. dirty            - Output changed, consumer hasn't observed (STALE)
4. potentially-stale- Producer updated, status uncertain (WARNING)
5. clean            - Synchronized (OK)
6. pending          - New edge, no data yet (INFO)
7. mock             - Using placeholder value (INFO)
```

**Visual Mapping:**

| Status | Color | Graph | Badge Background | Meaning |
|--------|-------|-------|------------------|---------|
| missing-output | Dark Red (#dc2626) | #dc2626 | bg-red-100 | Output removed from state |
| schema-invalid | Red (#ef4444) | #ef4444 | bg-red-100 | Output fails schema validation |
| dirty | Amber (#f59e0b) | #f59e0b | bg-orange-100 | Fingerprint mismatch |
| potentially-stale | Yellow (#eab308) | #f59e0b | bg-yellow-100 | Producer changed |
| pending | Blue (#3b82f6) | #3b82f6 | bg-blue-100 | Awaiting first observation |
| clean | Green (#10b981) | #10b981 | bg-green-100 | Synchronized |
| mock | Purple (#a855f7) | #6b7280 | bg-purple-100 | Using mock value |

### 5. Information Architecture Changes

#### Before (Current)

```
DetailView Modal
├─ Overview
│  ├─ Status
│  ├─ Size
│  ├─ Timestamps
│  ├─ Backend Config
│  └─ Outputs (simple grid)  ← Current location
├─ Labels
├─ Dependencies
└─ Dependents
```

#### After (Proposed)

```
DetailView Modal
├─ Overview
│  ├─ Status
│  ├─ Size
│  ├─ Timestamps
│  └─ Backend Config
├─ Outputs  ← NEW TAB
│  └─ Enhanced OutputCard components
├─ Labels
├─ Dependencies
└─ Dependents
```

**Rationale:**
- Outputs deserve dedicated tab due to schema complexity
- Overview tab remains focused on state-level metadata
- Follows existing pattern of dedicated tabs for collections (Dependencies, Dependents)
- Tab badge shows count: "Outputs (5)"

### 6. User Flows

#### 6.1 View Output Schema (Happy Path)

```
1. User clicks state in graph/list view
   → DetailView modal opens (Overview tab)

2. User clicks "Outputs" tab
   → See list of output cards

3. User sees vpc_id output with green checkmark icon
   → Preview shows: "type: string, pattern: ^vpc-[a-z0-9]+$"
   → Status: "✓ Schema validation passed"
   → Timestamp: "Validated 2 minutes ago"

4. User clicks "View Schema" button
   → Expandable section reveals formatted JSON Schema
   → Syntax highlighted in code block
```

#### 6.2 Investigate Schema Validation Failure

```
1. User notices red edge in graph view
   → Tooltip shows "schema-invalid"

2. User clicks producer state to open DetailView
   → Clicks "Outputs" tab

3. User sees subnet_ids output with orange warning icon
   → Red border around card
   → Error message: "value at index 0 does not match pattern"
   → Full validation error displayed

4. User clicks "View Schema" to understand expected format
   → Reviews JSON Schema pattern requirement
   → Can correlate error with schema constraint

5. User clicks "Dependents" tab
   → Sees which consumer states are affected
   → Red "schema-invalid" badges on edge cards
```

#### 6.3 Compare Outputs With/Without Schemas

```
1. User opens state with mixed outputs (some with schemas, some without)
   → Outputs tab shows all outputs

2. Outputs with schemas:
   → Show validation status
   → Display schema preview
   → Have "View Schema" button

3. Outputs without schemas:
   → Show "No schema defined" message
   → Gray border, no validation indicators
   → Only show output name and sensitive flag
```

### 7. Accessibility Considerations

#### Color Blindness

**Problem:** Red/Green indicators may not be distinguishable for users with protanopia/deuteranopia

**Solution:**
- ✅ Use **icons** in addition to colors (CheckCircle, AlertTriangle, XCircle)
- ✅ Use **text labels** ("Schema validation passed/failed")
- ✅ Use **border patterns** (solid border for valid, thicker for invalid)

#### Screen Readers

**Implementation:**

```tsx
<div
  className={`border rounded-lg ${getBorderColor()}`}
  role="region"
  aria-label={`Output ${output.key}${output.sensitive ? ' (sensitive)' : ''}${
    validationStatus === 'invalid' ? ' - Schema validation failed' : ''
  }`}
>
  {/* Card content */}
</div>
```

#### Keyboard Navigation

**Requirements:**
- ✅ "View Schema" button is focusable and activates on Enter/Space
- ✅ Tab order follows visual hierarchy (top to bottom)
- ✅ Focus indicator visible on all interactive elements

### 8. Performance Considerations

#### Schema JSON Parsing

**Concern:** Large JSON Schemas could slow rendering if parsed eagerly

**Solution:**
- Parse schema only when:
  1. Generating preview text (on initial render)
  2. User expands schema viewer (lazy parse)
- Use `useMemo` to cache parsed schemas:

```tsx
const parsedSchema = useMemo(() => {
  if (!output.schema_json) return null;
  try {
    return JSON.parse(output.schema_json);
  } catch {
    return null;
  }
}, [output.schema_json]);
```

#### Large Output Lists

**Concern:** States with 50+ outputs could make tab slow

**Solution:**
- Virtualized list rendering (future enhancement if needed)
- Currently not a concern - typical states have <20 outputs

### 9. Error Handling

#### Invalid JSON Schema

**Scenario:** Backend returns malformed `schema_json`

**Handling:**
```tsx
const parseSchemaPreview = (schemaJson: string): string => {
  try {
    const schema = JSON.parse(schemaJson);
    // ... build preview
  } catch {
    return 'Invalid JSON Schema';  // Graceful degradation
  }
};
```

#### Missing Validation Metadata

**Scenario:** Output has `schema_json` but no `validation_status`

**Handling:**
- Don't show validation indicators if `validation_status` is undefined
- Still show "View Schema" button
- Implies validation hasn't run yet (edge case during rollout)

### 10. Testing Strategy

#### Unit Tests

**File:** `webapp/src/components/__tests__/OutputCard.test.tsx` (NEW)

```typescript
describe('OutputCard', () => {
  it('renders output with valid schema', () => {
    const output: OutputKey = {
      key: 'vpc_id',
      sensitive: false,
      schema_json: '{"type":"string","pattern":"^vpc-"}',
      validation_status: 'valid',
      validated_at: new Date().toISOString(),
    };

    render(<OutputCard output={output} />);

    expect(screen.getByText('vpc_id')).toBeInTheDocument();
    expect(screen.getByText(/Schema validation passed/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /View Schema/i })).toBeInTheDocument();
  });

  it('renders output with invalid schema', () => {
    const output: OutputKey = {
      key: 'subnet_ids',
      sensitive: false,
      schema_json: '{"type":"array"}',
      validation_status: 'invalid',
      validation_error: 'Expected array, got string',
      validated_at: new Date().toISOString(),
    };

    render(<OutputCard output={output} />);

    expect(screen.getByText('subnet_ids')).toBeInTheDocument();
    expect(screen.getByText(/Schema validation failed/i)).toBeInTheDocument();
    expect(screen.getByText('Expected array, got string')).toBeInTheDocument();
  });

  it('renders output without schema', () => {
    const output: OutputKey = {
      key: 'availability_zones',
      sensitive: false,
    };

    render(<OutputCard output={output} />);

    expect(screen.getByText('availability_zones')).toBeInTheDocument();
    expect(screen.getByText(/No schema defined/i)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /View Schema/i })).not.toBeInTheDocument();
  });

  it('expands schema viewer on button click', async () => {
    const output: OutputKey = {
      key: 'vpc_id',
      sensitive: false,
      schema_json: '{"type":"string"}',
    };

    render(<OutputCard output={output} />);

    const button = screen.getByRole('button', { name: /View Schema/i });
    fireEvent.click(button);

    await waitFor(() => {
      expect(screen.getByText(/"type": "string"/)).toBeInTheDocument();
    });
  });
});
```

#### Integration Tests

**File:** `webapp/src/__tests__/dashboard_output_schema.test.tsx` (NEW)

```typescript
describe('Output Schema Display', () => {
  it('shows outputs tab with schema metadata', async () => {
    const mockState: StateInfo = {
      // ... mock state with outputs including schema_json
    };

    render(<DetailView state={mockState} onClose={jest.fn()} onNavigate={jest.fn()} />);

    // Click Outputs tab
    const outputsTab = screen.getByRole('button', { name: /Outputs/i });
    fireEvent.click(outputsTab);

    // Verify output cards render
    expect(screen.getByText('vpc_id')).toBeInTheDocument();
    expect(screen.getByText(/Schema validation passed/i)).toBeInTheDocument();
  });
});
```

#### Visual Regression Tests

**Tool:** Percy or Chromatic

**Scenarios:**
1. Output card - valid schema
2. Output card - invalid schema
3. Output card - no schema
4. Output card - expanded schema viewer
5. Outputs tab - mixed validation statuses
6. Edge list - schema-invalid badge
7. Graph view - schema-invalid edge color

### 11. Migration & Rollout

#### Phase 1: Backend Ready, Frontend Not Updated

**State:**
- Backend serves `schema_json`, `validation_status` fields
- Frontend ignores new fields
- Webapp continues showing outputs as before

**Impact:** None (backward compatible)

#### Phase 2: Frontend Updated, No Schemas Set

**State:**
- Frontend code deployed
- New OutputCard component renders
- All outputs show "No schema defined"

**Impact:** Users see new Outputs tab, but no schema data yet

#### Phase 3: Schemas Added via CLI

**State:**
- Users run `gridctl state set-output-schema`
- Backend stores schemas
- Webapp displays schema metadata on next refresh

**Impact:** Users see schemas appear in UI

#### Phase 4: Validation Enabled

**State:**
- Backend runs validation on tfstate POST
- Validation results populate
- Webapp shows validation status

**Impact:** Full feature active

### 12. Future Enhancements (Out of Scope)

#### 12.1 Schema Editor UI

**Feature:** Web form to create/edit JSON Schemas

**Rationale:** Currently webapp is read-only; editing requires this becomes read-write

#### 12.2 Bulk Operations

**Feature:** "Copy schema from another output" or "Apply schema to multiple outputs"

**Rationale:** Power users managing many outputs

#### 12.3 Schema Library

**Feature:** Catalog of common schemas (AWS VPC ID, subnet ID, region, etc.)

**Rationale:** Reduce duplication, promote consistency

#### 12.4 Validation History

**Feature:** Show past validation results over time (graph/timeline)

**Rationale:** Debugging why outputs became invalid

#### 12.5 Live Validation

**Feature:** WebSocket updates when validation status changes

**Rationale:** Currently requires manual refresh

## Summary of Changes

### Files to Modify

| File | Type | Changes |
|------|------|---------|
| `js/sdk/src/models/state-info.ts` | Type | Extend `OutputKey` interface, add `schema-invalid` to `EdgeStatus` |
| `js/sdk/src/adapter.ts` | SDK | Update `convertProtoOutputKey()` to map new fields |
| `webapp/src/components/DetailView.tsx` | Component | Add Outputs tab, remove outputs from Overview |
| `webapp/src/components/ListView.tsx` | Component | Add `schema-invalid` to badge styles |
| `webapp/src/components/graphflow/utils.ts` | Util | Add `schema-invalid` color to `getEdgeColor()` |

### Files to Create

| File | Type | Purpose |
|------|------|---------|
| `webapp/src/components/OutputCard.tsx` | Component | Enhanced output display with schema viewer |
| `webapp/src/components/__tests__/OutputCard.test.tsx` | Test | Unit tests for OutputCard |
| `webapp/src/__tests__/dashboard_output_schema.test.tsx` | Test | Integration tests for schema display |

### Estimated Effort

| Task | Effort | Priority |
|------|--------|----------|
| TypeScript model updates | 1 hour | P0 |
| OutputCard component | 4 hours | P0 |
| DetailView tab refactor | 2 hours | P0 |
| Edge status color updates | 1 hour | P0 |
| Unit tests | 3 hours | P1 |
| Integration tests | 2 hours | P1 |
| Visual regression tests | 2 hours | P2 |
| **Total** | **15 hours** | |

### Dependencies

- ✅ Backend API serves new fields (`schema_json`, `validation_status`, etc.)
- ✅ Protobuf types updated (`buf generate` run)
- ✅ TypeScript SDK regenerated
- ✅ No new NPM dependencies required (uses existing Lucide icons)

## Appendix: Color Palette Reference

### Tailwind CSS Classes Used

```css
/* Validation Status Colors */
.text-green-600    /* Valid - #059669 */
.bg-green-50       /* Valid background - #f0fdf4 */
.border-green-200  /* Valid border - #bbf7d0 */

.text-orange-600   /* Invalid - #ea580c */
.bg-orange-50      /* Invalid background - #fff7ed */
.border-orange-200 /* Invalid border - #fed7aa */

.text-red-600      /* Error - #dc2626 */
.bg-red-50         /* Error background - #fef2f2 */
.border-red-200    /* Error border - #fecaca */

/* Edge Status Colors (Graph) */
#10b981  /* green-500 - clean */
#f59e0b  /* amber-500 - dirty, potentially-stale */
#3b82f6  /* blue-500 - pending */
#ef4444  /* red-500 - schema-invalid */
#dc2626  /* red-600 - missing-output */
#6b7280  /* gray-500 - default/unknown */
```

## Appendix: Example API Response

**GET /state.v1.StateService/GetStateInfo**

```json
{
  "state": {
    "guid": "018c-abcd-1234-5678",
    "logic_id": "vpc-network-dev",
    "outputs": [
      {
        "key": "vpc_id",
        "sensitive": false,
        "schema_json": "{\"$schema\":\"http://json-schema.org/draft-07/schema#\",\"type\":\"string\",\"pattern\":\"^vpc-[a-z0-9]+$\"}",
        "validation_status": "valid",
        "validated_at": "2025-11-23T10:30:00Z"
      },
      {
        "key": "subnet_ids",
        "sensitive": false,
        "schema_json": "{\"type\":\"array\",\"items\":{\"type\":\"string\",\"pattern\":\"^subnet-\"}}",
        "validation_status": "invalid",
        "validation_error": "value at index 0 does not match pattern \"^subnet-\"",
        "validated_at": "2025-11-23T10:31:00Z"
      },
      {
        "key": "availability_zones",
        "sensitive": false
      }
    ],
    "dependents": [
      {
        "id": 42,
        "from_output": "vpc_id",
        "to_logic_id": "ec2-instances-dev",
        "status": "schema-invalid"
      }
    ]
  }
}
```
