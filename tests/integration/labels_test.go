package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/terraconstructs/grid/pkg/sdk"
)

func TestLabelsLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	workDir := t.TempDir()
	logicID := fmt.Sprintf("labels-lifecycle-%d", time.Now().UnixNano())

	mustRunGridctl(t, ctx, workDir, "state", "create", logicID)
	mustRunGridctl(t, ctx, "", "state", "set", logicID, "--label", "env=dev", "--label", "active=true")

	client := newSDKClient()
	include := true
	summaries, err := client.ListStatesWithOptions(ctx, sdk.ListStatesOptions{IncludeLabels: &include})
	require.NoError(t, err)

	var summary *sdk.StateSummary
	for i := range summaries {
		if summaries[i].LogicID == logicID {
			summary = &summaries[i]
			break
		}
	}
	require.NotNil(t, summary, "state summary not found")
	require.NotNil(t, summary.Labels)
	assert.Equal(t, "dev", summary.Labels["env"])
	assert.Equal(t, true, summary.Labels["active"])

	mustRunGridctl(t, ctx, "", "state", "set", logicID, "--label", "region=us-west", "--label", "-env")

	summaries, err = client.ListStatesWithOptions(ctx, sdk.ListStatesOptions{IncludeLabels: &include})
	require.NoError(t, err)
	summary = nil
	for i := range summaries {
		if summaries[i].LogicID == logicID {
			summary = &summaries[i]
			break
		}
	}
	require.NotNil(t, summary)
	require.NotNil(t, summary.Labels)
	_, envExists := summary.Labels["env"]
	assert.False(t, envExists)
	assert.Equal(t, "us-west", summary.Labels["region"])
	assert.Equal(t, true, summary.Labels["active"])
}

func TestLabelPolicyValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "policy.json")
	policyJSON := `{"max_keys":1,"max_value_len":256}`
	require.NoError(t, os.WriteFile(policyPath, []byte(policyJSON), 0644))

	mustRunGridctl(t, ctx, "", "policy", "set", "--file", policyPath)
	t.Cleanup(func() {
		defaultPolicy := filepath.Join(tmpDir, "policy-default.json")
		_ = os.WriteFile(defaultPolicy, []byte(`{"max_keys":32,"max_value_len":256}`), 0644)
		_, _ = runGridctl(t, context.Background(), "", "policy", "set", "--file", defaultPolicy)
	})

	logicID := fmt.Sprintf("labels-policy-%d", time.Now().UnixNano())
	mustRunGridctl(t, ctx, t.TempDir(), "state", "create", logicID)

	mustRunGridctl(t, ctx, "", "state", "set", logicID, "--label", "env=prod")

	output, err := runGridctl(t, ctx, "", "state", "set", logicID, "--label", "team=core")
	require.Error(t, err)
	assert.Contains(t, output, "max_keys")

	client := newSDKClient()
	include := true
	summaries, err := client.ListStatesWithOptions(ctx, sdk.ListStatesOptions{IncludeLabels: &include})
	require.NoError(t, err)
	var summary *sdk.StateSummary
	for i := range summaries {
		if summaries[i].LogicID == logicID {
			summary = &summaries[i]
			break
		}
	}
	require.NotNil(t, summary)
	assert.Equal(t, "prod", summary.Labels["env"])
}

func TestLabelUpdatesAtomic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logicID := fmt.Sprintf("labels-update-%d", time.Now().UnixNano())
	mustRunGridctl(t, ctx, t.TempDir(), "state", "create", logicID)
	mustRunGridctl(t, ctx, "", "state", "set", logicID, "--label", "env=dev", "--label", "team=core")

	mustRunGridctl(t, ctx, "", "state", "set", logicID, "--label", "region=us-east", "--label", "-team", "--label", "env=prod")

	client := newSDKClient()
	include := true
	summaries, err := client.ListStatesWithOptions(ctx, sdk.ListStatesOptions{IncludeLabels: &include})
	require.NoError(t, err)
	var summary *sdk.StateSummary
	for i := range summaries {
		if summaries[i].LogicID == logicID {
			summary = &summaries[i]
			break
		}
	}
	require.NotNil(t, summary)
	assert.Equal(t, "prod", summary.Labels["env"])
	assert.Equal(t, "us-east", summary.Labels["region"])
	_, teamExists := summary.Labels["team"]
	assert.False(t, teamExists)
}

func TestLabelFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logicProd := fmt.Sprintf("labels-prod-%d", time.Now().UnixNano())
	logicStaging := fmt.Sprintf("labels-staging-%d", time.Now().UnixNano())

	mustRunGridctl(t, ctx, t.TempDir(), "state", "create", logicProd)
	mustRunGridctl(t, ctx, "", "state", "set", logicProd, "--label", "env=prod", "--label", "region=us-west")
	mustRunGridctl(t, ctx, t.TempDir(), "state", "create", logicStaging)
	mustRunGridctl(t, ctx, "", "state", "set", logicStaging, "--label", "env=staging", "--label", "region=us-east")

	client := newSDKClient()
	include := true

	summaries, err := client.ListStatesWithOptions(ctx, sdk.ListStatesOptions{
		Filter:        "env == \"prod\"",
		IncludeLabels: &include,
	})
	require.NoError(t, err)
	require.NotEmpty(t, summaries)

	for _, summary := range summaries {
		assert.Equal(t, "prod", summary.Labels["env"])
	}

	labelFilter := sdk.BuildBexprFilter(sdk.LabelMap{"env": "staging"})
	summaries, err = client.ListStatesWithOptions(ctx, sdk.ListStatesOptions{
		Filter:        labelFilter,
		IncludeLabels: &include,
	})
	require.NoError(t, err)
	require.NotEmpty(t, summaries)
	for _, summary := range summaries {
		assert.Equal(t, "staging", summary.Labels["env"])
	}

	output := mustRunGridctl(t, ctx, "", "state", "list", "--filter", "env == \"prod\"")
	assert.Contains(t, output, logicProd)
	assert.NotContains(t, output, logicStaging)

	output = mustRunGridctl(t, ctx, "", "state", "list", "--label", "env=staging")
	assert.Contains(t, output, logicStaging)
	assert.NotContains(t, output, logicProd)
}

func TestLabelPolicyCompliance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tmpDir := t.TempDir()
	initialPolicyPath := filepath.Join(tmpDir, "policy-initial.json")
	initialPolicyJSON := `{"allowed_keys":{"env":{}},"allowed_values":{"env":["prod","dev"]},"max_keys":4,"max_value_len":256}`
	require.NoError(t, os.WriteFile(initialPolicyPath, []byte(initialPolicyJSON), 0644))

	mustRunGridctl(t, ctx, "", "policy", "set", "--file", initialPolicyPath)
	t.Cleanup(func() {
		defaultPolicy := filepath.Join(tmpDir, "policy-default.json")
		_ = os.WriteFile(defaultPolicy, []byte(`{"max_keys":32,"max_value_len":256}`), 0644)
		_, _ = runGridctl(t, context.Background(), "", "policy", "set", "--file", defaultPolicy)
	})

	validLogicID := fmt.Sprintf("labels-valid-%d", time.Now().UnixNano())
	invalidLogicID := fmt.Sprintf("labels-invalid-%d", time.Now().UnixNano())

	mustRunGridctl(t, ctx, t.TempDir(), "state", "create", validLogicID)
	mustRunGridctl(t, ctx, "", "state", "set", validLogicID, "--label", "env=prod")
	mustRunGridctl(t, ctx, t.TempDir(), "state", "create", invalidLogicID)
	mustRunGridctl(t, ctx, "", "state", "set", invalidLogicID, "--label", "env=dev")

	// Tighten policy to only allow env=prod
	restrictedPolicyPath := filepath.Join(tmpDir, "policy-restricted.json")
	restrictedPolicyJSON := `{"allowed_keys":{"env":{}},"allowed_values":{"env":["prod"]},"max_keys":4,"max_value_len":256}`
	require.NoError(t, os.WriteFile(restrictedPolicyPath, []byte(restrictedPolicyJSON), 0644))
	mustRunGridctl(t, ctx, "", "policy", "set", "--file", restrictedPolicyPath)

	output, err := runGridctl(t, ctx, "", "policy", "compliance")
	require.Error(t, err)
	assert.Contains(t, output, invalidLogicID)
	assert.Contains(t, output, "violate")

	mustRunGridctl(t, ctx, "", "state", "set", invalidLogicID, "--label", "env=prod")

	// Verify the invalid state was fixed - it should no longer appear in violations
	// Note: Other states from previous tests may still violate, which is expected
	output, _ = runGridctl(t, ctx, "", "policy", "compliance")
	assert.NotContains(t, output, invalidLogicID, "Fixed state should not appear in violations")
}
