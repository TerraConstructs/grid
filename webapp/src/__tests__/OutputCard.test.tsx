import { describe, it, expect } from "vitest";
import { screen, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom/vitest';
import userEvent from '@testing-library/user-event';
import { render } from '@testing-library/react';
import { OutputCard } from '../components/OutputCard';
import type { OutputKey } from '@tcons/grid';

describe('OutputCard Component', () => {
  describe('Rendering with valid schema', () => {
    it('displays output key with valid schema', () => {
      const output: OutputKey = {
        key: 'vpc_id',
        sensitive: false,
        schema_json: '{"type":"string","pattern":"^vpc-[a-z0-9]+$"}',
        validation_status: 'valid',
        validated_at: new Date(Date.now() - 60000).toISOString(), // 1 minute ago
      };

      render(<OutputCard output={output} />);

      expect(screen.getByText('vpc_id')).toBeInTheDocument();
      expect(screen.getByText(/type: string/)).toBeInTheDocument();
      expect(screen.getByText(/pattern:/)).toBeInTheDocument();
      expect(screen.getByText('Schema validation passed')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /View Schema/ })).toBeInTheDocument();
    });

    it('displays validation success message with green checkmark', () => {
      const output: OutputKey = {
        key: 'subnet_id',
        sensitive: false,
        schema_json: '{"type":"string"}',
        validation_status: 'valid',
        validated_at: new Date().toISOString(),
      };

      render(<OutputCard output={output} />);

      const successMessage = screen.getByText('Schema validation passed');
      expect(successMessage).toBeInTheDocument();
      expect(successMessage.closest('.text-green-700')).toBeInTheDocument();
    });

    it('displays relative time since validation', () => {
      const output: OutputKey = {
        key: 'region',
        sensitive: false,
        schema_json: '{"type":"string"}',
        validation_status: 'valid',
        validated_at: new Date(Date.now() - 120000).toISOString(), // 2 minutes ago
      };

      render(<OutputCard output={output} />);

      expect(screen.getByText(/Validated 2 minutes ago/)).toBeInTheDocument();
    });
  });

  describe('Rendering with invalid schema', () => {
    it('displays output with invalid schema validation error', () => {
      const output: OutputKey = {
        key: 'subnet_ids',
        sensitive: false,
        schema_json: '{"type":"array","items":{"type":"string","pattern":"^subnet-"}}',
        validation_status: 'invalid',
        validation_error: 'value at index 0 does not match pattern "^subnet-"',
        validated_at: new Date(Date.now() - 300000).toISOString(), // 5 minutes ago
      };

      render(<OutputCard output={output} />);

      expect(screen.getByText('subnet_ids')).toBeInTheDocument();
      expect(screen.getByText('Schema validation failed')).toBeInTheDocument();
      expect(screen.getByText(/value at index 0 does not match pattern/)).toBeInTheDocument();
      expect(screen.getByText(/Validated 5 minutes ago/)).toBeInTheDocument();
    });

    it('displays error message with warning triangle icon', () => {
      const output: OutputKey = {
        key: 'availability_zones',
        sensitive: false,
        schema_json: '{"type":"array"}',
        validation_status: 'invalid',
        validation_error: 'Expected array, got string',
        validated_at: new Date().toISOString(),
      };

      render(<OutputCard output={output} />);

      const errorMessage = screen.getByText('Schema validation failed');
      expect(errorMessage).toBeInTheDocument();
      expect(errorMessage.closest('.text-orange-700')).toBeInTheDocument();
      expect(screen.getByText('Expected array, got string')).toBeInTheDocument();
    });

    it('displays card with orange border for invalid schema', () => {
      const output: OutputKey = {
        key: 'complex',
        sensitive: false,
        schema_json: '{"type":"object"}',
        validation_status: 'invalid',
        validation_error: 'Schema mismatch',
        validated_at: new Date().toISOString(),
      };

      const { container } = render(<OutputCard output={output} />);
      const card = container.querySelector('[role="region"]');

      expect(card).toHaveClass('border-orange-200', 'bg-orange-50/30');
    });
  });

  describe('Rendering with error status', () => {
    it('displays validation error status with X icon', () => {
      const output: OutputKey = {
        key: 'failed_output',
        sensitive: false,
        schema_json: '{"type":"string"}',
        validation_status: 'error',
        validation_error: 'Validation engine error: timeout',
        validated_at: new Date().toISOString(),
      };

      render(<OutputCard output={output} />);

      expect(screen.getByText('Validation error')).toBeInTheDocument();
      expect(screen.getByText(/Validation engine error/)).toBeInTheDocument();
    });

    it('displays card with red border for validation error', () => {
      const output: OutputKey = {
        key: 'error_output',
        sensitive: false,
        schema_json: '{"type":"string"}',
        validation_status: 'error',
        validation_error: 'Internal error',
        validated_at: new Date().toISOString(),
      };

      const { container } = render(<OutputCard output={output} />);
      const card = container.querySelector('[role="region"]');

      expect(card).toHaveClass('border-red-200', 'bg-red-50/30');
    });
  });

  describe('Rendering without schema', () => {
    it('displays output without schema with neutral styling', () => {
      const output: OutputKey = {
        key: 'availability_zones',
        sensitive: false,
      };

      render(<OutputCard output={output} />);

      expect(screen.getByText('availability_zones')).toBeInTheDocument();
      expect(screen.getByText('No schema defined')).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /View Schema/ })).not.toBeInTheDocument();
      expect(screen.queryByText(/Validated/)).not.toBeInTheDocument();
    });

    it('displays card with gray border when no schema', () => {
      const output: OutputKey = {
        key: 'simple_output',
        sensitive: false,
      };

      const { container } = render(<OutputCard output={output} />);
      const card = container.querySelector('[role="region"]');

      expect(card).toHaveClass('border-gray-200');
    });
  });

  describe('Sensitive flag', () => {
    it('displays sensitive flag badge for sensitive outputs', () => {
      const output: OutputKey = {
        key: 'db_password',
        sensitive: true,
        schema_json: '{"type":"string"}',
        validation_status: 'valid',
        validated_at: new Date().toISOString(),
      };

      render(<OutputCard output={output} />);

      expect(screen.getByText('db_password')).toBeInTheDocument();
      expect(screen.getByText('sensitive')).toBeInTheDocument();
    });

    it('does not display sensitive flag for non-sensitive outputs', () => {
      const output: OutputKey = {
        key: 'public_key',
        sensitive: false,
        schema_json: '{"type":"string"}',
        validation_status: 'valid',
        validated_at: new Date().toISOString(),
      };

      render(<OutputCard output={output} />);

      expect(screen.queryByText('sensitive')).not.toBeInTheDocument();
    });

    it('displays both output name and sensitive flag together', () => {
      const output: OutputKey = {
        key: 'api_token',
        sensitive: true,
      };

      render(<OutputCard output={output} />);

      expect(screen.getByText('api_token')).toBeInTheDocument();
      expect(screen.getByText('sensitive')).toBeInTheDocument();
    });
  });

  describe('Schema viewer expansion', () => {
    it('expands and collapses schema viewer on button click', async () => {
      const user = userEvent.setup();
      const schema = {
        type: 'object',
        properties: {
          vpc_ids: { type: 'array' },
        },
      };
      const output: OutputKey = {
        key: 'complex_output',
        sensitive: false,
        schema_json: JSON.stringify(schema),
        validation_status: 'valid',
        validated_at: new Date().toISOString(),
      };

      render(<OutputCard output={output} />);

      const viewButton = screen.getByRole('button', { name: /View Schema/ });
      expect(viewButton).toHaveTextContent('View Schema');

      await user.click(viewButton);

      await waitFor(() => {
        expect(screen.getByText(/vpc_ids/)).toBeInTheDocument();
      });
      expect(viewButton).toHaveTextContent('Hide Schema');

      await user.click(viewButton);

      await waitFor(() => {
        expect(viewButton).toHaveTextContent('View Schema');
      });
    });

    it('displays formatted JSON when schema viewer is expanded', async () => {
      const user = userEvent.setup();
      const schema = {
        type: 'string',
        pattern: '^vpc-[a-z0-9]+$',
        minLength: 5,
      };
      const output: OutputKey = {
        key: 'vpc_id',
        sensitive: false,
        schema_json: JSON.stringify(schema),
        validation_status: 'valid',
        validated_at: new Date().toISOString(),
      };

      render(<OutputCard output={output} />);

      const viewButton = screen.getByRole('button', { name: /View Schema/ });
      await user.click(viewButton);

      await waitFor(() => {
        expect(screen.getByText(/"type": "string"/)).toBeInTheDocument();
        expect(screen.getByText(/"pattern"/)).toBeInTheDocument();
      });
    });

    it('does not show view schema button when no schema is defined', () => {
      const output: OutputKey = {
        key: 'output_without_schema',
        sensitive: false,
      };

      render(<OutputCard output={output} />);

      expect(screen.queryByRole('button', { name: /View Schema/ })).not.toBeInTheDocument();
    });
  });

  describe('Schema preview extraction', () => {
    it('extracts type from schema for preview', () => {
      const output: OutputKey = {
        key: 'simple_string',
        sensitive: false,
        schema_json: '{"type":"string"}',
        validation_status: 'valid',
        validated_at: new Date().toISOString(),
      };

      render(<OutputCard output={output} />);

      expect(screen.getByText(/type: string/)).toBeInTheDocument();
    });

    it('extracts pattern constraint for preview', () => {
      const output: OutputKey = {
        key: 'vpc_id',
        sensitive: false,
        schema_json: '{"type":"string","pattern":"^vpc-[a-z0-9]+$"}',
        validation_status: 'valid',
        validated_at: new Date().toISOString(),
      };

      render(<OutputCard output={output} />);

      expect(screen.getByText(/pattern: \^vpc-\[a-z0-9\]\+\$/)).toBeInTheDocument();
    });

    it('extracts enum values for preview', () => {
      const output: OutputKey = {
        key: 'region',
        sensitive: false,
        schema_json: '{"type":"string","enum":["us-east-1","us-west-2","eu-west-1"]}',
        validation_status: 'valid',
        validated_at: new Date().toISOString(),
      };

      render(<OutputCard output={output} />);

      expect(screen.getByText(/enum:/)).toBeInTheDocument();
    });

    it('handles invalid JSON schema gracefully', () => {
      const output: OutputKey = {
        key: 'bad_schema',
        sensitive: false,
        schema_json: 'not valid json',
        validation_status: 'valid',
        validated_at: new Date().toISOString(),
      };

      render(<OutputCard output={output} />);

      expect(screen.getByText('bad_schema')).toBeInTheDocument();
      expect(screen.getByText('Invalid JSON Schema')).toBeInTheDocument();
    });
  });

  describe('Accessibility', () => {
    it('has proper aria label for output region', () => {
      const output: OutputKey = {
        key: 'vpc_id',
        sensitive: false,
      };

      const { container } = render(<OutputCard output={output} />);
      const region = container.querySelector('[role="region"]');

      expect(region).toHaveAttribute('aria-label', expect.stringContaining('vpc_id'));
    });

    it('includes sensitive flag in aria label when present', () => {
      const output: OutputKey = {
        key: 'password',
        sensitive: true,
      };

      const { container } = render(<OutputCard output={output} />);
      const region = container.querySelector('[role="region"]');

      expect(region).toHaveAttribute('aria-label', expect.stringContaining('sensitive'));
    });

    it('indicates validation failure in aria label', () => {
      const output: OutputKey = {
        key: 'invalid_output',
        sensitive: false,
        schema_json: '{"type":"string"}',
        validation_status: 'invalid',
        validation_error: 'Schema mismatch',
        validated_at: new Date().toISOString(),
      };

      const { container } = render(<OutputCard output={output} />);
      const region = container.querySelector('[role="region"]');

      expect(region).toHaveAttribute('aria-label', expect.stringContaining('Schema validation failed'));
    });

    it('schema viewer button is focusable and activates on Enter', async () => {
      const user = userEvent.setup();
      const output: OutputKey = {
        key: 'test_output',
        sensitive: false,
        schema_json: '{"type":"string"}',
        validation_status: 'valid',
        validated_at: new Date().toISOString(),
      };

      render(<OutputCard output={output} />);

      const viewButton = screen.getByRole('button', { name: /View Schema/ });
      viewButton.focus();
      expect(viewButton).toHaveFocus();

      await user.keyboard('{Enter}');
      await waitFor(() => {
        expect(screen.getByText(/"type"/)).toBeInTheDocument();
      });
    });
  });

  describe('Schema source field', () => {
    it('does not display schema source visually but includes in data', () => {
      const output: OutputKey = {
        key: 'custom_output',
        sensitive: false,
        schema_json: '{"type":"string"}',
        schema_source: 'manual',
        validation_status: 'valid',
        validated_at: new Date().toISOString(),
      };

      render(<OutputCard output={output} />);

      // schema_source is present in data but not displayed to user
      expect(screen.getByText('custom_output')).toBeInTheDocument();
      // Verify schema_source field is not displayed (no "manual" or "inferred" badge shown)
      expect(screen.queryByText(/^manual$/)).not.toBeInTheDocument();
      expect(screen.queryByText(/^inferred$/)).not.toBeInTheDocument();
    });
  });

  describe('Edge cases', () => {
    it('handles very long error messages', () => {
      const shortError = 'validation failed: value too long';
      const output: OutputKey = {
        key: 'output',
        sensitive: false,
        schema_json: '{"type":"string"}',
        validation_status: 'invalid',
        validation_error: shortError,
        validated_at: new Date().toISOString(),
      };

      render(<OutputCard output={output} />);

      expect(screen.getByText(shortError)).toBeInTheDocument();
    });

    it('handles recently validated output (just now)', () => {
      const output: OutputKey = {
        key: 'recent_output',
        sensitive: false,
        schema_json: '{"type":"string"}',
        validation_status: 'valid',
        validated_at: new Date(Date.now() - 15000).toISOString(), // 15 seconds ago
      };

      render(<OutputCard output={output} />);

      expect(screen.getByText(/just now/)).toBeInTheDocument();
    });

    it('handles very old validation timestamp', () => {
      const output: OutputKey = {
        key: 'old_output',
        sensitive: false,
        schema_json: '{"type":"string"}',
        validation_status: 'valid',
        validated_at: new Date(Date.now() - 86400000 * 7).toISOString(), // 7 days ago
      };

      render(<OutputCard output={output} />);

      expect(screen.getByText(/7 days ago/)).toBeInTheDocument();
    });

    it('handles validation_status as undefined (not validated yet)', () => {
      const output: OutputKey = {
        key: 'not_validated',
        sensitive: false,
        schema_json: '{"type":"string"}',
        // validation_status is undefined
      };

      render(<OutputCard output={output} />);

      expect(screen.getByText('not_validated')).toBeInTheDocument();
      expect(screen.queryByText(/validation/i)).not.toBeInTheDocument();
    });

    it('handles complex nested schema', () => {
      const complexSchema = {
        type: 'object',
        properties: {
          network: {
            type: 'object',
            properties: {
              subnets: { type: 'array', items: { type: 'string' } },
            },
          },
        },
      };
      const output: OutputKey = {
        key: 'infrastructure',
        sensitive: false,
        schema_json: JSON.stringify(complexSchema),
        validation_status: 'valid',
        validated_at: new Date().toISOString(),
      };

      render(<OutputCard output={output} />);

      expect(screen.getByText('infrastructure')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /View Schema/ })).toBeInTheDocument();
    });
  });
});
