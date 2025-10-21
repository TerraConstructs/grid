package terraform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// FindTerraformBinary discovers the Terraform or OpenTofu binary to use.
// It follows this precedence:
//  1. tfBinOverride parameter (from --tf-bin flag)
//  2. TERRAFORM_BINARY_NAME environment variable
//  3. Auto-detect: search for "terraform", then "tofu" in PATH
//
// Returns the absolute path to the binary, or an error if no binary is found
// or if the binary is not executable.
//
// Reference: specs/006-authz-authn-rbac/plan.md §743-747, FR-097b, FR-097h
func FindTerraformBinary(tfBinOverride string) (string, error) {
	if tfBinOverride != "" {
		return validateBinary(tfBinOverride)
	}

	if envBin := os.Getenv("TERRAFORM_BINARY_NAME"); envBin != "" {
		return validateBinary(envBin)
	}

	for _, binName := range []string{"terraform", "tofu"} {
		path, err := exec.LookPath(binName)
		if err == nil {
			// Found the binary, validate it's executable
			if validPath, err := validateBinary(path); err == nil {
				return validPath, nil
			}
		}
	}

	// No binary found
	return "", errors.New("no Terraform or OpenTofu binary found: install terraform or tofu, set TERRAFORM_BINARY_NAME environment variable or use --tf-bin flag")
}

// validateBinary checks that the binary exists and is executable.
// Returns the absolute path to the binary, or an error if validation fails.
func validateBinary(path string) (string, error) {
	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("binary not found: %s", path)
		}
		return "", fmt.Errorf("failed to stat binary %s: %w", path, err)
	}

	// Check if it's a regular file (not a directory)
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a binary: %s", path)
	}

	// Check if executable (Unix permission check)
	// On Unix systems, we check if any execute bit is set
	if info.Mode()&0111 == 0 {
		return "", fmt.Errorf("binary is not executable: %s", path)
	}

	// Return the path as-is (exec.LookPath already returns absolute path)
	return path, nil
}
