package sdk

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	statev1 "github.com/terraconstructs/grid/api/state/v1"
)

// LabelMap stores typed label values exposed by the SDK.
type LabelMap map[string]any

// Label represents a single key/value label pair after sorting.
type Label struct {
	Key   string
	Value any
}

// ConvertProtoLabels converts a proto label map into a LabelMap with native Go types.
func ConvertProtoLabels(pb map[string]*statev1.LabelValue) LabelMap {
	if len(pb) == 0 {
		return nil
	}
	labels := make(LabelMap, len(pb))
	for key, val := range pb {
		labels[key] = labelValueFromProto(val)
	}
	return labels
}

// ConvertLabelsToProto converts a LabelMap to the proto representation used by RPC requests.
func ConvertLabelsToProto(labels LabelMap) map[string]*statev1.LabelValue {
	if len(labels) == 0 {
		return nil
	}
	pb := make(map[string]*statev1.LabelValue, len(labels))
	for key, value := range labels {
		pb[key] = labelValueToProto(value)
	}
	return pb
}

// SortLabels returns a slice of Label sorted lexicographically by key.
// Nil input results in an empty slice.
func SortLabels(labels LabelMap) []Label {
	if len(labels) == 0 {
		return []Label{}
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	sorted := make([]Label, 0, len(keys))
	for _, key := range keys {
		sorted = append(sorted, Label{Key: key, Value: labels[key]})
	}
	return sorted
}

// BuildBexprFilter builds a bexpr AND filter from the provided labels.
// Strings are quoted, booleans and numbers are emitted verbatim.
// When labels is empty an empty string is returned.
func BuildBexprFilter(labels LabelMap) string {
	if len(labels) == 0 {
		return ""
	}
	expressions := make([]string, 0, len(labels))
	for _, label := range SortLabels(labels) {
		expressions = append(expressions, fmt.Sprintf("%s == %s", label.Key, formatBexprValue(label.Value)))
	}
	return strings.Join(expressions, " && ")
}

func labelValueFromProto(val *statev1.LabelValue) any {
	if val == nil {
		return nil
	}
	switch v := val.Value.(type) {
	case *statev1.LabelValue_StringValue:
		return v.StringValue
	case *statev1.LabelValue_NumberValue:
		return v.NumberValue
	case *statev1.LabelValue_BoolValue:
		return v.BoolValue
	default:
		return nil
	}
}

func labelValueToProto(value any) *statev1.LabelValue {
	switch v := value.(type) {
	case *statev1.LabelValue:
		return v
	case string:
		return &statev1.LabelValue{Value: &statev1.LabelValue_StringValue{StringValue: v}}
	case fmt.Stringer:
		return &statev1.LabelValue{Value: &statev1.LabelValue_StringValue{StringValue: v.String()}}
	case bool:
		return &statev1.LabelValue{Value: &statev1.LabelValue_BoolValue{BoolValue: v}}
	case int:
		return &statev1.LabelValue{Value: &statev1.LabelValue_NumberValue{NumberValue: float64(v)}}
	case int32:
		return &statev1.LabelValue{Value: &statev1.LabelValue_NumberValue{NumberValue: float64(v)}}
	case int64:
		return &statev1.LabelValue{Value: &statev1.LabelValue_NumberValue{NumberValue: float64(v)}}
	case uint:
		return &statev1.LabelValue{Value: &statev1.LabelValue_NumberValue{NumberValue: float64(v)}}
	case uint32:
		return &statev1.LabelValue{Value: &statev1.LabelValue_NumberValue{NumberValue: float64(v)}}
	case uint64:
		return &statev1.LabelValue{Value: &statev1.LabelValue_NumberValue{NumberValue: float64(v)}}
	case float32:
		return &statev1.LabelValue{Value: &statev1.LabelValue_NumberValue{NumberValue: float64(v)}}
	case float64:
		return &statev1.LabelValue{Value: &statev1.LabelValue_NumberValue{NumberValue: v}}
	default:
		return &statev1.LabelValue{Value: &statev1.LabelValue_StringValue{StringValue: fmt.Sprintf("%v", value)}}
	}
}

func formatBexprValue(value any) string {
	switch v := value.(type) {
	case string:
		return strconv.Quote(v)
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
	case float32:
		return formatBexprValue(float64(v))
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	default:
		return strconv.Quote(fmt.Sprintf("%v", value))
	}
}
