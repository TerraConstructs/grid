package state

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/terraconstructs/grid/pkg/sdk"
)

const labelPreviewLimit = 32

func parseLabelArgs(args []string) (sdk.LabelMap, []string, error) {
	labels := sdk.LabelMap{}
	warnings := []string{}

	for _, raw := range args {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		parts := strings.SplitN(raw, "=", 2)
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("invalid label format %q (expected key=value)", raw)
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, nil, fmt.Errorf("label key cannot be empty (%q)", raw)
		}

		if _, exists := labels[key]; exists {
			warnings = append(warnings, fmt.Sprintf("duplicate label %q detected, last value wins", key))
		}

		labels[key] = inferLabelValue(val)
	}

	return labels, warnings, nil
}

func parseLabelMutations(args []string) (sdk.LabelMap, []string, []string, error) {
	addArgs := []string{}
	removals := []string{}

	for _, raw := range args {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		if strings.HasPrefix(raw, "-") && !strings.Contains(raw, "=") {
			key := strings.TrimPrefix(raw, "-")
			key = strings.TrimSpace(key)
			if key == "" {
				return nil, nil, nil, fmt.Errorf("label removal flag %q missing key", raw)
			}
			removals = append(removals, key)
			continue
		}

		addArgs = append(addArgs, raw)
	}

	adds, warnings, err := parseLabelArgs(addArgs)
	if err != nil {
		return nil, nil, nil, err
	}

	return adds, removals, warnings, nil
}

func inferLabelValue(raw string) any {
	lower := strings.ToLower(raw)
	if b, err := strconv.ParseBool(lower); err == nil {
		return b
	}

	if i, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return float64(i)
	}

	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return f
	}

	return raw
}

func formatLabelPreview(labels sdk.LabelMap) string {
	if len(labels) == 0 {
		return "-"
	}

	pairs := make([]string, 0, len(labels))
	for _, label := range sdk.SortLabels(labels) {
		pairs = append(pairs, fmt.Sprintf("%s=%s", label.Key, labelValueToString(label.Value)))
	}

	joined := strings.Join(pairs, ",")
	if len(joined) <= labelPreviewLimit {
		return joined
	}
	return joined[:labelPreviewLimit] + "..."
}

func labelValueToString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64:
		if _, frac := math.Modf(v); frac == 0 {
			return strconv.FormatFloat(v, 'f', 0, 64)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", value)
	}
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}
