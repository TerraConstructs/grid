package tfstate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOutputs_Empty(t *testing.T) {
	outputs, err := ParseOutputs([]byte{})
	require.NoError(t, err)
	assert.Empty(t, outputs)
}

func TestParseOutputs_SimpleOutputs(t *testing.T) {
	tfstate := `{
		"outputs": {
			"vpc_id": {
				"value": "vpc-12345",
				"sensitive": false
			},
			"subnet_id": {
				"value": "subnet-67890"
			}
		}
	}`

	outputs, err := ParseOutputs([]byte(tfstate))
	require.NoError(t, err)
	assert.Len(t, outputs, 2)
	assert.Equal(t, "vpc-12345", outputs["vpc_id"])
	assert.Equal(t, "subnet-67890", outputs["subnet_id"])
}

func TestParseOutputs_ComplexTypes(t *testing.T) {
	tfstate := `{
		"outputs": {
			"instance_ids": {
				"value": ["i-123", "i-456", "i-789"]
			},
			"metadata": {
				"value": {
					"region": "us-west-2",
					"environment": "prod",
					"count": 3
				}
			},
			"port": {
				"value": 8080
			},
			"enabled": {
				"value": true
			}
		}
	}`

	outputs, err := ParseOutputs([]byte(tfstate))
	require.NoError(t, err)
	assert.Len(t, outputs, 4)

	// Array
	instanceIDs, ok := outputs["instance_ids"].([]interface{})
	require.True(t, ok)
	assert.Len(t, instanceIDs, 3)
	assert.Equal(t, "i-123", instanceIDs[0])

	// Object
	metadata, ok := outputs["metadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "us-west-2", metadata["region"])
	assert.Equal(t, "prod", metadata["environment"])
	assert.Equal(t, float64(3), metadata["count"]) // JSON numbers are float64

	// Number
	assert.Equal(t, float64(8080), outputs["port"])

	// Boolean
	assert.Equal(t, true, outputs["enabled"])
}

func TestParseOutputs_SensitiveOutputs(t *testing.T) {
	tfstate := `{
		"outputs": {
			"db_password": {
				"value": "super-secret-password",
				"sensitive": true
			},
			"public_key": {
				"value": "ssh-rsa AAAAB3...",
				"sensitive": false
			}
		}
	}`

	outputs, err := ParseOutputs([]byte(tfstate))
	require.NoError(t, err)
	assert.Len(t, outputs, 2)

	// Sensitive flag is ignored; we extract the value
	assert.Equal(t, "super-secret-password", outputs["db_password"])
	assert.Equal(t, "ssh-rsa AAAAB3...", outputs["public_key"])
}

func TestParseOutputs_NoOutputs(t *testing.T) {
	tfstate := `{
		"outputs": {}
	}`

	outputs, err := ParseOutputs([]byte(tfstate))
	require.NoError(t, err)
	assert.Empty(t, outputs)
}

func TestParseOutputs_MissingOutputsKey(t *testing.T) {
	tfstate := `{
		"version": 4,
		"resources": []
	}`

	outputs, err := ParseOutputs([]byte(tfstate))
	require.NoError(t, err)
	assert.Empty(t, outputs)
}

func TestParseOutputs_InvalidJSON(t *testing.T) {
	tfstate := `{invalid json`

	outputs, err := ParseOutputs([]byte(tfstate))
	assert.Error(t, err)
	assert.Nil(t, outputs)
	assert.Contains(t, err.Error(), "failed to parse tfstate JSON")
}

func TestParseOutputs_MalformedStructure(t *testing.T) {
	// outputs is not an object
	tfstate := `{
		"outputs": "invalid"
	}`

	outputs, err := ParseOutputs([]byte(tfstate))
	assert.Error(t, err)
	assert.Nil(t, outputs)
	assert.Contains(t, err.Error(), "failed to parse tfstate JSON")
}

func TestParseOutputs_NullValue(t *testing.T) {
	tfstate := `{
		"outputs": {
			"nullable_output": {
				"value": null
			}
		}
	}`

	outputs, err := ParseOutputs([]byte(tfstate))
	require.NoError(t, err)
	assert.Len(t, outputs, 1)
	assert.Nil(t, outputs["nullable_output"])
}

func TestParseOutputs_RealWorldExample(t *testing.T) {
	tfstate := `{
		"version": 4,
		"terraform_version": "1.5.0",
		"serial": 42,
		"lineage": "abc-123",
		"outputs": {
			"vpc_id": {
				"value": "vpc-0a1b2c3d4e5f6g7h8",
				"type": "string"
			},
			"private_subnet_ids": {
				"value": [
					"subnet-0a1b2c3d",
					"subnet-4e5f6g7h",
					"subnet-8i9j0k1l"
				],
				"type": ["list", "string"]
			},
			"nat_gateway_ips": {
				"value": {
					"us-east-1a": "52.1.2.3",
					"us-east-1b": "52.4.5.6"
				},
				"type": ["map", "string"]
			}
		},
		"resources": [
			{
				"mode": "managed",
				"type": "aws_vpc",
				"name": "main"
			}
		]
	}`

	outputs, err := ParseOutputs([]byte(tfstate))
	require.NoError(t, err)
	assert.Len(t, outputs, 3)

	assert.Equal(t, "vpc-0a1b2c3d4e5f6g7h8", outputs["vpc_id"])

	subnets, ok := outputs["private_subnet_ids"].([]interface{})
	require.True(t, ok)
	assert.Len(t, subnets, 3)

	natIPs, ok := outputs["nat_gateway_ips"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "52.1.2.3", natIPs["us-east-1a"])
}
