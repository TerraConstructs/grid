package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/terraconstructs/grid/pkg/sdk"
)

// T022: Test duplicate --label flag handling
func TestStateCreateCommand_DuplicateLabelFlags(t *testing.T) {
	t.Run("last value wins for duplicate keys", func(t *testing.T) {
		// Simulate multiple --label flags with same key
		labels := []string{
			"env=staging",
			"team=core",
			"env=prod", // Duplicate key, should override staging
		}

		parsedLabels, warnings, err := parseLabelArgs(labels)
		assert.NoError(t, err)

		// Last value should win
		assert.Equal(t, "prod", parsedLabels["env"], "Last env value should win")
		assert.Equal(t, "core", parsedLabels["team"])

		// User should be informed
		assert.NotEmpty(t, warnings, "Should have warning about duplicate")
		assert.Contains(t, warnings[0], "env")
		assert.Contains(t, warnings[0], "duplicate")
	})

	t.Run("no warnings for unique keys", func(t *testing.T) {
		labels := []string{
			"env=staging",
			"team=core",
			"region=us-west",
		}

		parsedLabels, warnings, err := parseLabelArgs(labels)
		assert.NoError(t, err)

		assert.Len(t, parsedLabels, 3)
		assert.Empty(t, warnings, "Should have no warnings for unique keys")
	})

	t.Run("multiple duplicates generate multiple warnings", func(t *testing.T) {
		labels := []string{
			"env=staging",
			"env=prod",
			"team=core",
			"team=platform",
		}

		_, warnings, err := parseLabelArgs(labels)
		assert.NoError(t, err)

		assert.Len(t, warnings, 2, "Should warn for both env and team")
	})
}

func TestParseLabelMutations(t *testing.T) {
	adds, removals, warnings, err := parseLabelMutations([]string{"env=prod", "-team"})
	assert.NoError(t, err)
	assert.Equal(t, "prod", adds["env"])
	assert.Equal(t, []string{"team"}, removals)
	assert.Empty(t, warnings)

	_, _, _, err = parseLabelMutations([]string{"invalid"})
	assert.Error(t, err)
}

func TestFormatLabelPreview(t *testing.T) {
	labels := sdk.LabelMap{"env": "staging", "region": "us-west"}
	preview := formatLabelPreview(labels)
	assert.Contains(t, preview, "env=staging")
	assert.True(t, len(preview) <= labelPreviewLimit+3)
}
