import type { LabelScalar } from '../models/state-info.js';

type BexprValue = LabelScalar;

function escapeString(value: string): string {
  return value.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
}

function formatValue(value: BexprValue): string {
  if (typeof value === 'string') {
    return `"${escapeString(value)}"`;
  }
  if (typeof value === 'boolean') {
    return value ? 'true' : 'false';
  }
  return Number(value).toString();
}

function wrapExpression(expr: string): string {
  const trimmed = expr.trim();
  if (trimmed === '') {
    return '';
  }
  if (trimmed.startsWith('(') && trimmed.endsWith(')')) {
    return trimmed;
  }
  return `(${trimmed})`;
}

export function buildEqualityFilter(key: string, value: BexprValue): string {
  return `${key} == ${formatValue(value)}`;
}

export function buildInFilter(key: string, values: BexprValue[]): string {
  if (values.length === 0) {
    return '';
  }
  if (values.length === 1) {
    return buildEqualityFilter(key, values[0]);
  }
  // bexpr doesn't support "field in [array]" syntax
  // Generate OR expression instead: key == val1 or key == val2 or ...
  // Wrap in parentheses to ensure correct precedence when combined with AND
  const conditions = values.map(val => buildEqualityFilter(key, val));
  const combined = combineFilters(conditions, 'OR');
  return `(${combined})`;
}

export function combineFilters(
  filters: string[],
  operator: 'AND' | 'OR' = 'AND'
): string {
  const cleaned = filters.map((f) => f.trim()).filter(Boolean);
  if (cleaned.length === 0) {
    return '';
  }
  if (cleaned.length === 1) {
    return cleaned[0];
  }

  const normalized = operator.toUpperCase() === 'OR' ? 'OR' : 'AND';
  const joiner = normalized === 'OR' ? ' or ' : ' and ';
  return cleaned.map(wrapExpression).join(joiner);
}
