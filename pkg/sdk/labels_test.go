package sdk

import (
	"testing"

	statev1 "github.com/terraconstructs/grid/api/state/v1"
)

func TestConvertProtoLabels(t *testing.T) {
	proto := map[string]*statev1.LabelValue{
		"env":    {Value: &statev1.LabelValue_StringValue{StringValue: "prod"}},
		"cost":   {Value: &statev1.LabelValue_NumberValue{NumberValue: 42}},
		"active": {Value: &statev1.LabelValue_BoolValue{BoolValue: true}},
	}

	labels := ConvertProtoLabels(proto)
	if len(labels) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(labels))
	}
	if labels["env"].(string) != "prod" {
		t.Fatalf("expected env=prod, got %v", labels["env"])
	}
	if labels["cost"].(float64) != 42 {
		t.Fatalf("expected cost=42, got %v", labels["cost"])
	}
	if labels["active"].(bool) != true {
		t.Fatalf("expected active=true, got %v", labels["active"])
	}
}

func TestConvertLabelsToProto(t *testing.T) {
	labels := LabelMap{
		"team":   "platform",
		"weight": 3,
	}

	proto := ConvertLabelsToProto(labels)
	if len(proto) != 2 {
		t.Fatalf("expected 2 proto labels, got %d", len(proto))
	}
	if proto["team"].GetStringValue() != "platform" {
		t.Fatalf("expected team=platform, got %v", proto["team"].GetStringValue())
	}
	if proto["weight"].GetNumberValue() != 3 {
		t.Fatalf("expected weight=3, got %v", proto["weight"].GetNumberValue())
	}
}

func TestSortLabels(t *testing.T) {
	labels := LabelMap{"b": 2, "a": 1}
	sorted := SortLabels(labels)
	if len(sorted) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(sorted))
	}
	if sorted[0].Key != "a" || sorted[1].Key != "b" {
		t.Fatalf("labels not sorted: %#v", sorted)
	}
}

func TestBuildBexprFilter(t *testing.T) {
	labels := LabelMap{"env": "staging", "active": true}
	filter := BuildBexprFilter(labels)
	expected := "active == true && env == \"staging\""
	if filter != expected {
		t.Fatalf("expected filter %q, got %q", expected, filter)
	}
}
