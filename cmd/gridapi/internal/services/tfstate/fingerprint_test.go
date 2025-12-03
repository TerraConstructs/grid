package tfstate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeFingerprint_Primitives(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  string
	}{
		{
			name:  "nil",
			value: nil,
			want:  ComputeFingerprint(nil),
		},
		{
			name:  "true",
			value: true,
			want:  ComputeFingerprint(true),
		},
		{
			name:  "false",
			value: false,
			want:  ComputeFingerprint(false),
		},
		{
			name:  "string",
			value: "vpc-12345",
			want:  ComputeFingerprint("vpc-12345"),
		},
		{
			name:  "int",
			value: 42,
			want:  ComputeFingerprint(42),
		},
		{
			name:  "float",
			value: 3.14159,
			want:  ComputeFingerprint(3.14159),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp := ComputeFingerprint(tt.value)
			assert.NotEmpty(t, fp)
			assert.Equal(t, tt.want, fp)
		})
	}
}

func TestComputeFingerprint_Deterministic(t *testing.T) {
	value := "vpc-12345"

	fp1 := ComputeFingerprint(value)
	fp2 := ComputeFingerprint(value)

	assert.Equal(t, fp1, fp2, "fingerprints should be deterministic")
}

func TestComputeFingerprint_DifferentValues(t *testing.T) {
	fp1 := ComputeFingerprint("vpc-12345")
	fp2 := ComputeFingerprint("vpc-67890")

	assert.NotEqual(t, fp1, fp2, "different values should produce different fingerprints")
}

func TestComputeFingerprint_Arrays(t *testing.T) {
	arr1 := []interface{}{"a", "b", "c"}
	arr2 := []interface{}{"a", "b", "c"}
	arr3 := []interface{}{"c", "b", "a"} // Different order

	fp1 := ComputeFingerprint(arr1)
	fp2 := ComputeFingerprint(arr2)
	fp3 := ComputeFingerprint(arr3)

	assert.Equal(t, fp1, fp2, "identical arrays should have same fingerprint")
	assert.NotEqual(t, fp1, fp3, "arrays with different order should have different fingerprints")
}

func TestComputeFingerprint_NestedArrays(t *testing.T) {
	arr := []interface{}{
		[]interface{}{"subnet-1", "subnet-2"},
		[]interface{}{"subnet-3", "subnet-4"},
	}

	fp := ComputeFingerprint(arr)
	assert.NotEmpty(t, fp)

	// Same structure should produce same fingerprint
	arr2 := []interface{}{
		[]interface{}{"subnet-1", "subnet-2"},
		[]interface{}{"subnet-3", "subnet-4"},
	}
	fp2 := ComputeFingerprint(arr2)
	assert.Equal(t, fp, fp2)
}

func TestComputeFingerprint_Objects(t *testing.T) {
	obj1 := map[string]interface{}{
		"vpc_id":    "vpc-123",
		"subnet_id": "subnet-456",
	}

	obj2 := map[string]interface{}{
		"subnet_id": "subnet-456",
		"vpc_id":    "vpc-123",
	}

	fp1 := ComputeFingerprint(obj1)
	fp2 := ComputeFingerprint(obj2)

	assert.Equal(t, fp1, fp2, "objects with same keys/values in different order should have same fingerprint")
}

func TestComputeFingerprint_NestedObjects(t *testing.T) {
	obj := map[string]interface{}{
		"network": map[string]interface{}{
			"vpc_id":    "vpc-123",
			"subnet_id": "subnet-456",
		},
		"metadata": map[string]interface{}{
			"region": "us-west-2",
			"zone":   "us-west-2a",
		},
	}

	fp := ComputeFingerprint(obj)
	assert.NotEmpty(t, fp)

	// Same structure should produce same fingerprint
	obj2 := map[string]interface{}{
		"metadata": map[string]interface{}{
			"zone":   "us-west-2a",
			"region": "us-west-2",
		},
		"network": map[string]interface{}{
			"subnet_id": "subnet-456",
			"vpc_id":    "vpc-123",
		},
	}
	fp2 := ComputeFingerprint(obj2)
	assert.Equal(t, fp, fp2, "nested objects with same keys/values should have same fingerprint regardless of order")
}

func TestComputeFingerprint_MixedTypes(t *testing.T) {
	complex := map[string]interface{}{
		"string_val": "test",
		"int_val":    42,
		"float_val":  3.14,
		"bool_val":   true,
		"array_val":  []interface{}{1, 2, 3},
		"object_val": map[string]interface{}{
			"nested": "value",
		},
		"null_val": nil,
	}

	fp := ComputeFingerprint(complex)
	assert.NotEmpty(t, fp)

	// Verify deterministic
	fp2 := ComputeFingerprint(complex)
	assert.Equal(t, fp, fp2)
}

func TestComputeFingerprint_EmptyStructures(t *testing.T) {
	emptyArray := []interface{}{}
	emptyObject := map[string]interface{}{}

	fpArray := ComputeFingerprint(emptyArray)
	fpObject := ComputeFingerprint(emptyObject)

	assert.NotEmpty(t, fpArray)
	assert.NotEmpty(t, fpObject)
	assert.NotEqual(t, fpArray, fpObject, "empty array and empty object should have different fingerprints")
}

func TestComputeFingerprint_NilValue(t *testing.T) {
	fp := ComputeFingerprint(nil)
	assert.NotEmpty(t, fp)

	// Verify nil is treated consistently
	fp2 := ComputeFingerprint(nil)
	assert.Equal(t, fp, fp2)
}

func TestComputeFingerprint_RealWorldVPCExample(t *testing.T) {
	vpcOutput := map[string]interface{}{
		"vpc_id":     "vpc-0a1b2c3d4e5f6g7h8",
		"cidr_block": "10.0.0.0/16",
		"private_subnets": []interface{}{
			"subnet-0a1b2c3d",
			"subnet-4e5f6g7h",
			"subnet-8i9j0k1l",
		},
		"nat_gateway_ips": map[string]interface{}{
			"us-east-1a": "52.1.2.3",
			"us-east-1b": "52.4.5.6",
		},
	}

	fp := ComputeFingerprint(vpcOutput)
	assert.NotEmpty(t, fp)

	// Modify a value
	vpcOutput["vpc_id"] = "vpc-different"
	fp2 := ComputeFingerprint(vpcOutput)
	assert.NotEqual(t, fp, fp2, "modified value should produce different fingerprint")
}

func TestCompareFingerprints(t *testing.T) {
	fp1 := ComputeFingerprint("value1")
	fp2 := ComputeFingerprint("value1")
	fp3 := ComputeFingerprint("value2")

	assert.True(t, CompareFingerprints(fp1, fp2), "identical fingerprints should compare equal")
	assert.False(t, CompareFingerprints(fp1, fp3), "different fingerprints should not compare equal")
	assert.False(t, CompareFingerprints("", fp1), "empty fingerprint should not compare equal")
	assert.False(t, CompareFingerprints(fp1, ""), "comparing with empty should return false")
	assert.False(t, CompareFingerprints("", ""), "two empty fingerprints should not compare equal")
}

func TestFormatFingerprint(t *testing.T) {
	tests := []struct {
		name        string
		fingerprint string
		want        string
	}{
		{
			name:        "long fingerprint",
			fingerprint: "AbCdEfGhIjKlMnOpQrStUvWxYz",
			want:        "AbCdEfGhIjKl...",
		},
		{
			name:        "exactly 12 chars",
			fingerprint: "123456789012",
			want:        "123456789012",
		},
		{
			name:        "short fingerprint",
			fingerprint: "abc",
			want:        "abc",
		},
		{
			name:        "empty",
			fingerprint: "",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatFingerprint(tt.fingerprint)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestComputeFingerprint_Base58Encoding(t *testing.T) {
	// Verify fingerprint uses Base58 encoding (no 0, O, I, l characters)
	fp := ComputeFingerprint("test-value")
	assert.NotContains(t, fp, "0")
	assert.NotContains(t, fp, "O")
	assert.NotContains(t, fp, "I")
	assert.NotContains(t, fp, "l")
}

func TestCanonicalJSON_KeyOrdering(t *testing.T) {
	// Verify that object keys are always sorted
	obj1 := map[string]interface{}{
		"z_last":  "value",
		"a_first": "value",
		"m_mid":   "value",
	}

	obj2 := map[string]interface{}{
		"m_mid":   "value",
		"z_last":  "value",
		"a_first": "value",
	}

	fp1 := ComputeFingerprint(obj1)
	fp2 := ComputeFingerprint(obj2)

	assert.Equal(t, fp1, fp2, "key ordering should be consistent regardless of map iteration order")
}

func TestComputeFingerprint_NumberPrecision(t *testing.T) {
	// Test that float precision doesn't cause fingerprint drift
	f1 := 1.23456789
	f2 := 1.23456789

	fp1 := ComputeFingerprint(f1)
	fp2 := ComputeFingerprint(f2)

	assert.Equal(t, fp1, fp2)

	// Different precision should produce different fingerprint
	f3 := 1.2345678
	fp3 := ComputeFingerprint(f3)
	assert.NotEqual(t, fp1, fp3)
}
