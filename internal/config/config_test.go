package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irootkernel/manta/internal/model"
)

func TestValidateAcceptsImplementedParsers(t *testing.T) {
	t.Parallel()
	for _, parser := range []string{"generic", "vitest", "pytest", "go-test", "playwright"} {
		parser := parser
		t.Run(parser, func(t *testing.T) {
			t.Parallel()
			cfg := model.Config{
				Version: 2,
				Commands: map[string]model.CommandConfig{
					"unit": {
						Command:    []string{"sh", "test.sh"},
						Tags:       []string{"unit"},
						Parser:     parser,
						TimeoutSec: 60,
					},
				},
			}
			if err := Validate(cfg); err != nil {
				t.Fatalf("expected parser %q config to validate, got %v", parser, err)
			}
		})
	}
}

func TestLoadRejectsUnknownFieldsAndMultipleDocuments(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		data string
	}{
		{
			name: "unknown field",
			data: strings.Join([]string{
				"version: 2",
				"redactions:",
				"  patterns: []",
			}, "\n") + "\n",
		},
		{
			name: "multiple documents",
			data: "version: 2\n---\nversion: 2\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			path := filepath.Join(repo, "tester.yaml")
			if err := os.WriteFile(path, []byte(test.data), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, _, err := Load(repo, path, false); err == nil {
				t.Fatalf("expected %s to fail closed", test.name)
			}
		})
	}
}

func TestValidateRejectsUnknownParser(t *testing.T) {
	t.Parallel()
	cfg := model.Config{
		Version: 2,
		Commands: map[string]model.CommandConfig{
			"unit": {
				Command:    []string{"sh", "test.sh"},
				Tags:       []string{"unit"},
				Parser:     "made-up",
				TimeoutSec: 60,
			},
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected unknown parser to fail validation")
	}
}

func TestValidateRejectsUnsafeCommandIDs(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"../unit", "/tmp/unit", "nested/unit", ".", "유닛"} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			cfg := model.Config{
				Version: 2,
				Commands: map[string]model.CommandConfig{
					id: {
						Command:    []string{"true"},
						Tags:       []string{"unit"},
						Parser:     "generic",
						TimeoutSec: 60,
					},
				},
			}
			if err := Validate(cfg); err == nil {
				t.Fatalf("expected unsafe command id %q to fail", id)
			}
		})
	}
}

func TestLoadCanonicalizesTags(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	path := filepath.Join(repo, "tester.yaml")
	data := strings.Join([]string{
		"version: 2",
		"commands:",
		"  unit:",
		"    command: [go, test, ./...]",
		"    tags: [unit, go, unit]",
		"    parser: go-test",
		"    timeout_sec: 60",
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(repo, path, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(cfg.Commands["unit"].Tags, ","); got != "go,unit" {
		t.Fatalf("canonical tags = %q, want go,unit", got)
	}
}

func TestValidateRejectsMissingAndUnsafeTags(t *testing.T) {
	t.Parallel()
	for _, tags := range [][]string{nil, {}, {"unit/test"}, {" unit"}} {
		cfg := model.Config{
			Version: 2,
			Commands: map[string]model.CommandConfig{
				"unit": {Command: []string{"true"}, Tags: tags, Parser: "generic", TimeoutSec: 60},
			},
		}
		if err := Validate(cfg); err == nil {
			t.Fatalf("expected tags %q to fail validation", tags)
		}
	}
}

func TestValidateRejectsVersionOne(t *testing.T) {
	t.Parallel()
	if err := Validate(model.Config{Version: 1}); err == nil {
		t.Fatal("expected config version 1 to be rejected")
	}
}
