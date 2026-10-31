package oci_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"oracle-fisher/internal/oci"
)

func TestTransientNetworkError(t *testing.T) {
	if oci.TransientNetworkError(nil) {
		t.Errorf("expected false for nil error")
	}

	testCases := []struct {
		err      error
		expected bool
	}{
		{errors.New("dial tcp: i/o timeout"), true},
		{errors.New("context deadline exceeded: timeout exceeded"), true},
		{errors.New("read: connection reset by peer"), true},
		{errors.New("dial tcp: connection refused"), true},
		{errors.New("remote error: tls handshake timeout"), true},
		{errors.New("out of host capacity"), false},
		{errors.New("NotAuthorizedOrNotFound"), false},
	}

	for _, tc := range testCases {
		res := oci.TransientNetworkError(tc.err)
		if res != tc.expected {
			t.Errorf("for error '%v', expected %v, got %v", tc.err, tc.expected, res)
		}
	}
}

func TestPublicSSHKey(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test_key.pub")
	keyContent := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQC... user@test"

	if err := os.WriteFile(keyPath, []byte(keyContent+"\n"), 0644); err != nil {
		t.Fatalf("failed to write test key file: %v", err)
	}

	res := oci.PublicSSHKey(keyPath)
	if res != keyContent {
		t.Errorf("expected '%s', got '%s'", keyContent, res)
	}

	// Missing file should return fallback or empty
	missingRes := oci.PublicSSHKey(filepath.Join(tmpDir, "non_existent.pub"))
	if missingRes != "" {
		t.Errorf("expected empty string for non-existent key path, got '%s'", missingRes)
	}
}
