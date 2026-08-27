package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApiTokenMissingNamesDocumentedPath(t *testing.T) {
	if v, ok := os.LookupEnv("BUFFER_API_KEY"); ok {
		defer os.Setenv("BUFFER_API_KEY", v)
	} else {
		defer os.Unsetenv("BUFFER_API_KEY")
	}
	os.Unsetenv("BUFFER_API_KEY")

	_, err := apiToken()
	if err == nil {
		t.Fatal("apiToken() returned no error with BUFFER_API_KEY unset")
	}
	msg := err.Error()
	if !strings.Contains(msg, "BUFFER_API_KEY") {
		t.Errorf("error %q does not name BUFFER_API_KEY", msg)
	}
	if !strings.Contains(msg, ".env") {
		t.Errorf("error %q does not name the documented .env path", msg)
	}
	if strings.Contains(msg, "-key") || strings.Contains(msg, "--key") {
		t.Errorf("error %q suggests a flag, which bfr must never support", msg)
	}
}

func TestApiTokenReadsFromEnv(t *testing.T) {
	if v, ok := os.LookupEnv("BUFFER_API_KEY"); ok {
		defer os.Setenv("BUFFER_API_KEY", v)
	} else {
		defer os.Unsetenv("BUFFER_API_KEY")
	}
	os.Setenv("BUFFER_API_KEY", "test-token-value")

	tok, err := apiToken()
	if err != nil {
		t.Fatalf("apiToken() returned error with BUFFER_API_KEY set: %v", err)
	}
	if tok != "test-token-value" {
		t.Errorf("apiToken() = %q, want %q", tok, "test-token-value")
	}
}

func TestLoadEnvFileParsesAndSkipsCommentsAndBlanks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "# comment line\n\nFOO_BAR=baz\nQUOTED=\"with quotes\"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write test .env: %v", err)
	}

	for _, key := range []string{"FOO_BAR", "QUOTED"} {
		if v, ok := os.LookupEnv(key); ok {
			defer os.Setenv(key, v)
		} else {
			defer os.Unsetenv(key)
		}
		os.Unsetenv(key)
	}

	loadEnvFile(path)

	if got := os.Getenv("FOO_BAR"); got != "baz" {
		t.Errorf("FOO_BAR = %q, want %q", got, "baz")
	}
	if got := os.Getenv("QUOTED"); got != "with quotes" {
		t.Errorf("QUOTED = %q, want %q", got, "with quotes")
	}
}

func TestLoadEnvFileNeverOverridesExistingEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("BUFFER_API_KEY=from-file\n"), 0o600); err != nil {
		t.Fatalf("failed to write test .env: %v", err)
	}

	if v, ok := os.LookupEnv("BUFFER_API_KEY"); ok {
		defer os.Setenv("BUFFER_API_KEY", v)
	} else {
		defer os.Unsetenv("BUFFER_API_KEY")
	}
	os.Setenv("BUFFER_API_KEY", "already-exported")

	loadEnvFile(path)

	if got := os.Getenv("BUFFER_API_KEY"); got != "already-exported" {
		t.Errorf("BUFFER_API_KEY = %q, want unchanged %q", got, "already-exported")
	}
}

func TestLoadEnvFileMissingFileIsNotAnError(t *testing.T) {
	loadEnvFile(filepath.Join(t.TempDir(), "does-not-exist.env"))
}

func TestEnsureEnvSourcesDotEnvFromWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("BUFFER_API_KEY=from-dot-env\n"), 0o600); err != nil {
		t.Fatalf("failed to write test .env: %v", err)
	}

	if v, ok := os.LookupEnv("BUFFER_API_KEY"); ok {
		defer os.Setenv("BUFFER_API_KEY", v)
	} else {
		defer os.Unsetenv("BUFFER_API_KEY")
	}
	os.Unsetenv("BUFFER_API_KEY")

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd failed: %v", err)
	}
	defer os.Chdir(origWd)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir failed: %v", err)
	}

	ensureEnv()

	if got := os.Getenv("BUFFER_API_KEY"); got != "from-dot-env" {
		t.Errorf("BUFFER_API_KEY = %q, want %q (ensureEnv did not read repo-root .env)", got, "from-dot-env")
	}
}

func TestEnsureEnvLeavesAlreadyExportedKeyAlone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("BUFFER_API_KEY=from-dot-env\n"), 0o600); err != nil {
		t.Fatalf("failed to write test .env: %v", err)
	}

	if v, ok := os.LookupEnv("BUFFER_API_KEY"); ok {
		defer os.Setenv("BUFFER_API_KEY", v)
	} else {
		defer os.Unsetenv("BUFFER_API_KEY")
	}
	os.Setenv("BUFFER_API_KEY", "already-exported")

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd failed: %v", err)
	}
	defer os.Chdir(origWd)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir failed: %v", err)
	}

	ensureEnv()

	if got := os.Getenv("BUFFER_API_KEY"); got != "already-exported" {
		t.Errorf("BUFFER_API_KEY = %q, want unchanged %q", got, "already-exported")
	}
}
