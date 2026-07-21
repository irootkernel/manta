package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/model"
	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/safety"
)

const DefaultConfigPath = ".kkachi/tester.yaml"

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
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && allowMissing && override == "" {
			cfg := model.Config{Version: 1, Commands: map[string]model.CommandConfig{}}
			return cfg, path, nil
		}
		return model.Config{}, path, model.NewKATError(model.ExitCodeConfigError, "read config", err)
	}

	var cfg model.Config
	if err := safety.DecodeYAMLStrict(data, &cfg); err != nil {
		return model.Config{}, path, model.NewKATError(model.ExitCodeConfigError, "parse config", err)
	}
	if cfg.Commands == nil {
		cfg.Commands = map[string]model.CommandConfig{}
	}
	if err := Validate(cfg); err != nil {
		return model.Config{}, path, err
	}
	return cfg, path, nil
}

func Validate(cfg model.Config) error {
	if cfg.Version != 1 {
		return model.NewKATError(model.ExitCodeConfigError, "validate config", fmt.Errorf("unsupported config version %d", cfg.Version))
	}
	for id, cmd := range cfg.Commands {
		if err := safety.ValidateArtifactIdentifier("command id", id); err != nil {
			return model.NewKATError(model.ExitCodeConfigError, "validate config", err)
		}
		if len(cmd.Command) == 0 {
			return model.NewKATError(model.ExitCodeConfigError, "validate config", fmt.Errorf("command %q must define argv command", id))
		}
		for _, part := range cmd.Command {
			if strings.TrimSpace(part) == "" {
				return model.NewKATError(model.ExitCodeConfigError, "validate config", fmt.Errorf("command %q contains empty argv item", id))
			}
		}
		if strings.TrimSpace(cmd.Lane) == "" {
			return model.NewKATError(model.ExitCodeConfigError, "validate config", fmt.Errorf("command %q must define lane", id))
		}
		if err := validateParserLabel(cmd.Parser, true); err != nil {
			return err
		}
		if cmd.TimeoutSec <= 0 || cmd.TimeoutSec > 86400 {
			return model.NewKATError(model.ExitCodeConfigError, "validate config", fmt.Errorf("command %q timeout_sec must be between 1 and 86400", id))
		}
	}
	for _, pattern := range cfg.Redaction.Patterns {
		if strings.TrimSpace(pattern.Name) == "" {
			return model.NewKATError(model.ExitCodeConfigError, "validate config", fmt.Errorf("redaction pattern name must not be empty"))
		}
		if _, err := regexp.Compile(pattern.Regex); err != nil {
			return model.NewKATError(model.ExitCodeConfigError, "validate config", fmt.Errorf("redaction pattern %q invalid regex: %w", pattern.Name, err))
		}
	}
	return nil
}

func ValidateAdHocLane(lane string) error {
	if strings.TrimSpace(lane) == "" {
		return model.NewKATError(model.ExitCodeConfigError, "validate ad hoc lane", fmt.Errorf("--lane is required for ad-hoc runs"))
	}
	return nil
}

func validateParserLabel(label string, requireSupported bool) error {
	if !slices.Contains(knownParsers, label) {
		return model.NewKATError(model.ExitCodeConfigError, "validate parser", fmt.Errorf("unsupported parser label %q", label))
	}
	_ = requireSupported
	return nil
}

func ValidateParserLabel(label string) error {
	return validateParserLabel(label, true)
}
