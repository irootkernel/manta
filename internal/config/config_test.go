package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/model"
)

func TestValidateAcceptsImplementedParsers(t *testing.T) {
	t.Parallel()
	for _, parser := range []string{"generic", "vitest", "pytest", "go-test", "playwright"} {
		parser := parser
		t.Run(parser, func(t *testing.T) {
			t.Parallel()
			cfg := model.Config{
				Version: 1,
				Commands: map[string]model.CommandConfig{
					"unit": {
						Command:    []string{"sh", "test.sh"},
						Lane:       "unit",
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
				"version: 1",
				"redactions:",
				"  patterns: []",
			}, "\n") + "\n",
		},
		{
			name: "multiple documents",
			data: "version: 1\n---\nversion: 1\n",
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
		Version: 1,
		Commands: map[string]model.CommandConfig{
			"unit": {
				Command:    []string{"sh", "test.sh"},
				Lane:       "unit",
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
				Version: 1,
				Commands: map[string]model.CommandConfig{
					id: {
						Command:    []string{"true"},
						Lane:       "unit",
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
