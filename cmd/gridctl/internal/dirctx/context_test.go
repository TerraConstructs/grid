package dirctx

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func chdirTemp(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	return tmp
}

func TestValidateValidContext(t *testing.T) {
	dc := &DirectoryContext{
		Version:      GridFileVersion,
		StateGUID:    "0199039d-8b5e-7a2f-b7c4-1a2b3c4d5e6f", // valid uuid v7-like
		StateLogicID: "logic-id",
		ServerURL:    "http://localhost:8080",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := dc.Validate(); err != nil {
		t.Fatalf("expected valid context, got error: %v", err)
	}
}

func TestValidateErrors(t *testing.T) {
	cases := []struct {
		name string
		ctx  DirectoryContext
	}{
		{"wrong_version", DirectoryContext{Version: "999", StateGUID: "0199039d-8b5e-7a2f-b7c4-1a2b3c4d5e6f", StateLogicID: "x"}},
		{"missing_guid", DirectoryContext{Version: GridFileVersion, StateLogicID: "x"}},
		{"bad_guid", DirectoryContext{Version: GridFileVersion, StateGUID: "not-a-guid", StateLogicID: "x"}},
		{"missing_logic", DirectoryContext{Version: GridFileVersion, StateGUID: "0199039d-8b5e-7a2f-b7c4-1a2b3c4d5e6f"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.ctx.Validate(); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestReadGridContextMissingReturnsNil(t *testing.T) {
	chdirTemp(t)
	ctx, err := ReadGridContext()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx != nil {
		t.Fatalf("expected nil context when .grid missing")
	}
}

func TestWriteAndReadGridContext_RoundTrip(t *testing.T) {
	tmp := chdirTemp(t)
	now := time.Now().UTC()
	dc := &DirectoryContext{
		Version:      GridFileVersion,
		StateGUID:    "0199039d-8b5e-7a2f-b7c4-1a2b3c4d5e6f",
		StateLogicID: "logic-rt",
		ServerURL:    "http://example",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := WriteGridContext(dc); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Ensure file exists at cwd/.grid
	if _, err := os.Stat(filepath.Join(tmp, GridFileName)); err != nil {
		t.Fatalf(".grid not written: %v", err)
	}

	got, err := ReadGridContext()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got == nil {
		t.Fatalf("expected non-nil context")
	}
	if got.StateGUID != dc.StateGUID || got.StateLogicID != dc.StateLogicID || got.Version != GridFileVersion {
		t.Fatalf("mismatch after round trip: %+v vs %+v", got, dc)
	}
}

func TestWriteGridContextRejectsInvalid(t *testing.T) {
	chdirTemp(t)
	dc := &DirectoryContext{ // missing required fields
		Version: "bad",
	}
	if err := WriteGridContext(dc); err == nil {
		t.Fatalf("expected error writing invalid context")
	}
}

func TestReadGridContextCorruptedJSON(t *testing.T) {
	chdirTemp(t)
	if err := os.WriteFile(GridFileName, []byte("{not-json}"), 0644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	if ctx, err := ReadGridContext(); err == nil || ctx != nil {
		t.Fatalf("expected error and nil context for corrupt JSON")
	}
}

func TestReadGridContextInvalidVersion(t *testing.T) {
	chdirTemp(t)
	payload := []byte("{\n  \"version\": \"999\",\n  \"state_guid\": \"0199039d-8b5e-7a2f-b7c4-1a2b3c4d5e6f\",\n  \"state_logic_id\": \"x\"\n}\n")
	if err := os.WriteFile(GridFileName, payload, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if ctx, err := ReadGridContext(); err == nil || ctx != nil {
		t.Fatalf("expected error and nil context for invalid version")
	}
}

func TestResolveStateRef(t *testing.T) {
	// explicit wins
	e := StateRef{LogicID: "e"}
	c := StateRef{LogicID: "c"}
	got, err := ResolveStateRef(e, c)
	if err != nil || got.LogicID != "e" {
		t.Fatalf("explicit should win, got=%+v err=%v", got, err)
	}
	// fallback to context
	got, err = ResolveStateRef(StateRef{}, c)
	if err != nil || got.LogicID != "c" {
		t.Fatalf("context should be used, got=%+v err=%v", got, err)
	}
	// error when both empty
	if _, err := ResolveStateRef(StateRef{}, StateRef{}); err == nil {
		t.Fatalf("expected error when neither ref provided")
	}
}

func TestStateRefHelpers(t *testing.T) {
	if !(StateRef{}).IsEmpty() {
		t.Fatalf("empty ref should be empty")
	}
	if (StateRef{GUID: "g"}).IsEmpty() {
		t.Fatalf("non-empty ref should not be empty")
	}
	if got := (StateRef{LogicID: "x"}).String(); got != "x" {
		t.Fatalf("string should be logic-id, got %q", got)
	}
}

// Note: The JSON layout and trailing newline are implementation details
// and not asserted here to keep tests focused on the command-level contract.

func TestDirectoryContext_GetGUID(t *testing.T) {
	dc := &DirectoryContext{
		StateGUID: "0199039d-8b5e-7a2f-b7c4-1a2b3c4d5e6f",
	}
	if got := dc.GetGUID(); got != "0199039d-8b5e-7a2f-b7c4-1a2b3c4d5e6f" {
		t.Fatalf("GetGUID() = %q, want %q", got, "0199039d-8b5e-7a2f-b7c4-1a2b3c4d5e6f")
	}
}

func TestDirectoryContext_GetBackendURL(t *testing.T) {
	dc := &DirectoryContext{
		StateGUID: "0199039d-8b5e-7a2f-b7c4-1a2b3c4d5e6f",
		ServerURL: "http://localhost:8080",
	}
	want := "http://localhost:8080/tfstate/0199039d-8b5e-7a2f-b7c4-1a2b3c4d5e6f"
	if got := dc.GetBackendURL(); got != want {
		t.Fatalf("GetBackendURL() = %q, want %q", got, want)
	}
}

func TestDirectoryContext_GetLockURL(t *testing.T) {
	dc := &DirectoryContext{
		StateGUID: "0199039d-8b5e-7a2f-b7c4-1a2b3c4d5e6f",
		ServerURL: "http://localhost:8080",
	}
	want := "http://localhost:8080/tfstate/0199039d-8b5e-7a2f-b7c4-1a2b3c4d5e6f/lock"
	if got := dc.GetLockURL(); got != want {
		t.Fatalf("GetLockURL() = %q, want %q", got, want)
	}
}

func TestDirectoryContext_GetUnlockURL(t *testing.T) {
	dc := &DirectoryContext{
		StateGUID: "0199039d-8b5e-7a2f-b7c4-1a2b3c4d5e6f",
		ServerURL: "http://localhost:8080",
	}
	want := "http://localhost:8080/tfstate/0199039d-8b5e-7a2f-b7c4-1a2b3c4d5e6f/unlock"
	if got := dc.GetUnlockURL(); got != want {
		t.Fatalf("GetUnlockURL() = %q, want %q", got, want)
	}
}

func TestDirectoryContext_BackendURLs_WithHTTPS(t *testing.T) {
	dc := &DirectoryContext{
		StateGUID: "0199c24c-b330-79ee-9b25-4bb80926868f",
		ServerURL: "https://grid.example.com",
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "backend_url",
			got:  dc.GetBackendURL(),
			want: "https://grid.example.com/tfstate/0199c24c-b330-79ee-9b25-4bb80926868f",
		},
		{
			name: "lock_url",
			got:  dc.GetLockURL(),
			want: "https://grid.example.com/tfstate/0199c24c-b330-79ee-9b25-4bb80926868f/lock",
		},
		{
			name: "unlock_url",
			got:  dc.GetUnlockURL(),
			want: "https://grid.example.com/tfstate/0199c24c-b330-79ee-9b25-4bb80926868f/unlock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}
