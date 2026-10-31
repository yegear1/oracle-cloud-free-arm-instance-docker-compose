package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"oracle-fisher/internal/config"
)

func TestGet(t *testing.T) {
	key := "TEST_CONFIG_GET_VAR"
	os.Unsetenv(key)

	if val := config.Get(key, "default_val"); val != "default_val" {
		t.Fatalf("expected 'default_val', got '%s'", val)
	}

	os.Setenv(key, "custom_val")
	defer os.Unsetenv(key)

	if val := config.Get(key, "default_val"); val != "custom_val" {
		t.Fatalf("expected 'custom_val', got '%s'", val)
	}
}

func TestGetFloat(t *testing.T) {
	key := "TEST_CONFIG_FLOAT_VAR"
	os.Unsetenv(key)

	if val := config.GetFloat(key, 2.5); val != 2.5 {
		t.Fatalf("expected 2.5, got %v", val)
	}

	os.Setenv(key, "4.0")
	defer os.Unsetenv(key)

	if val := config.GetFloat(key, 2.5); val != 4.0 {
		t.Fatalf("expected 4.0, got %v", val)
	}

	// Invalid float should fallback to default
	os.Setenv(key, "not-a-number")
	if val := config.GetFloat(key, 2.5); val != 2.5 {
		t.Fatalf("expected default 2.5 on invalid number, got %v", val)
	}
}

func TestGetInt(t *testing.T) {
	key := "TEST_CONFIG_INT_VAR"
	os.Unsetenv(key)

	if val := config.GetInt(key, 100); val != 100 {
		t.Fatalf("expected 100, got %v", val)
	}

	os.Setenv(key, "200")
	defer os.Unsetenv(key)

	if val := config.GetInt(key, 100); val != 200 {
		t.Fatalf("expected 200, got %v", val)
	}

	// Invalid int should fallback to default
	os.Setenv(key, "invalid")
	if val := config.GetInt(key, 100); val != 100 {
		t.Fatalf("expected default 100 on invalid int, got %v", val)
	}
}

func TestLoadEnv(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env.test")

	content := `
# Comment line
TEST_KEY_ONE="value_one"
TEST_KEY_TWO='value_two'
TEST_KEY_THREE=value_three
`
	if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to create temp env file: %v", err)
	}

	defer func() {
		os.Unsetenv("TEST_KEY_ONE")
		os.Unsetenv("TEST_KEY_TWO")
		os.Unsetenv("TEST_KEY_THREE")
	}()

	config.LoadEnv(envPath)

	if val := config.Get("TEST_KEY_ONE", ""); val != "value_one" {
		t.Errorf("expected 'value_one', got '%s'", val)
	}
	if val := config.Get("TEST_KEY_TWO", ""); val != "value_two" {
		t.Errorf("expected 'value_two', got '%s'", val)
	}
	if val := config.Get("TEST_KEY_THREE", ""); val != "value_three" {
		t.Errorf("expected 'value_three', got '%s'", val)
	}
}
