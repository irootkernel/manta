package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/irootkernel/manta/internal/model"
	"github.com/irootkernel/manta/internal/safety"
	"github.com/irootkernel/manta/internal/tagset"
)

const DefaultConfigPath = ".manta/tester.yaml"

var knownParsers = []string{"generic", "vitest", "pytest", "go-test", "playwright"}

func ResolveConfigPath(repoRoot, override string) string {
	if override != "" {
		if filepath.IsAbs(override) {
			return override
		}
		return filepath.Join(repoRoot, override)
	}
	return filepath.Join(repoRoot, DefaultConfigPath)
}

func Load(repoRoot, override string, allowMissing bool) (model.Config, string, error) {
	path := ResolveConfigPath(repoRoot, override)
	data, err := safety.ReadFileLimited(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && allowMissing && override == "" {
			cfg := model.Config{Version: 2, Commands: map[string]model.CommandConfig{}}
			return cfg, path, nil
		}
		return model.Config{}, path, model.NewMantaError(model.ExitCodeConfigError, "read config", err)
	}

	var cfg model.Config
	if err := safety.DecodeYAMLStrict(data, &cfg); err != nil {
		return model.Config{}, path, model.NewMantaError(model.ExitCodeConfigError, "parse config", err)
	}
	if cfg.Commands == nil {
		cfg.Commands = map[string]model.CommandConfig{}
	}
	if err := Validate(cfg); err != nil {
		return model.Config{}, path, err
	}
	for id, cmd := range cfg.Commands {
		cmd.Tags = tagset.Normalize(cmd.Tags)
		cfg.Commands[id] = cmd
	}
	return cfg, path, nil
}

func Validate(cfg model.Config) error {
	if cfg.Version != 2 {
		return model.NewMantaError(model.ExitCodeConfigError, "validate config", fmt.Errorf("unsupported config version %d", cfg.Version))
	}
	for id, cmd := range cfg.Commands {
		if err := safety.ValidateArtifactIdentifier("command id", id); err != nil {
			return model.NewMantaError(model.ExitCodeConfigError, "validate config", err)
		}
		if len(cmd.Command) == 0 {
			return model.NewMantaError(model.ExitCodeConfigError, "validate config", fmt.Errorf("command %q must define argv command", id))
		}
		for _, part := range cmd.Command {
			if strings.TrimSpace(part) == "" {
				return model.NewMantaError(model.ExitCodeConfigError, "validate config", fmt.Errorf("command %q contains empty argv item", id))
			}
		}
		if err := tagset.Validate(cmd.Tags); err != nil {
			return model.NewMantaError(model.ExitCodeConfigError, "validate config", fmt.Errorf("command %q tags: %w", id, err))
		}
		if err := validateParserLabel(cmd.Parser, true); err != nil {
			return err
		}
		if cmd.TimeoutSec <= 0 || cmd.TimeoutSec > 86400 {
			return model.NewMantaError(model.ExitCodeConfigError, "validate config", fmt.Errorf("command %q timeout_sec must be between 1 and 86400", id))
		}
	}
	for _, pattern := range cfg.Redaction.Patterns {
		if strings.TrimSpace(pattern.Name) == "" {
			return model.NewMantaError(model.ExitCodeConfigError, "validate config", fmt.Errorf("redaction pattern name must not be empty"))
		}
		if _, err := regexp.Compile(pattern.Regex); err != nil {
			return model.NewMantaError(model.ExitCodeConfigError, "validate config", fmt.Errorf("redaction pattern %q invalid regex: %w", pattern.Name, err))
		}
	}
	return nil
}

func ValidateTags(values []string, operation string) ([]string, error) {
	canonical, err := tagset.Canonicalize(values)
	if err != nil {
		return nil, model.NewMantaError(model.ExitCodeConfigError, operation, err)
	}
	return canonical, nil
}

func validateParserLabel(label string, requireSupported bool) error {
	if !slices.Contains(knownParsers, label) {
		return model.NewMantaError(model.ExitCodeConfigError, "validate parser", fmt.Errorf("unsupported parser label %q", label))
	}
	_ = requireSupported
	return nil
}

func ValidateParserLabel(label string) error {
	return validateParserLabel(label, true)
}
