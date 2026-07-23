package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestVersionHumanOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Main([]string{"--version"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if got, want := stdout.String(), "manta v0.1.5\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestVersionJSONOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	info := BuildInfo{Name: "manta", Version: "1.2.3", Commit: "abc123", BuildDate: "2026-01-01T00:00:00Z"}

	exitCode := Run([]string{"version", "--json"}, &stdout, &stderr, info)
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}

	var payload versionOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	want := versionOutput{Name: "manta", Version: "1.2.3"}
	if payload != want {
		t.Fatalf("payload = %#v, want %#v", payload, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestUnsupportedGlobalOptionsFailClosed(t *testing.T) {
	for _, option := range []string{"--verbose", "--no-color"} {
		t.Run(option, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			exitCode := Main([]string{option, "--version"}, &stdout, &stderr)
			if exitCode != 2 {
				t.Fatalf("exitCode = %d, want 2", exitCode)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), "flag provided but not defined") {
				t.Fatalf("stderr = %q, want unsupported-option diagnostic", stderr.String())
			}
		})
	}
}
