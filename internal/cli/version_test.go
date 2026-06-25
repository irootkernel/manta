package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestVersionHumanOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Main([]string{"--version"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if got, want := stdout.String(), "kkachi-agent-tester 0.1.0\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestVersionJSONOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	info := BuildInfo{Name: "kkachi-agent-tester", Version: "1.2.3", Commit: "abc123", BuildDate: "2026-01-01T00:00:00Z"}

	exitCode := Run([]string{"version", "--json"}, &stdout, &stderr, info)
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}

	var payload BuildInfo
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if payload != info {
		t.Fatalf("payload = %#v, want %#v", payload, info)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
