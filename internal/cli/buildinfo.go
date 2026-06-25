package cli

import (
	"regexp"
	"runtime/debug"
	"strings"
)

const devVersion = "0.0.0-dev"

var pseudoModuleVersionPattern = regexp.MustCompile(`^v?[0-9]+[.][0-9]+[.][0-9]+(-[0-9]{14}-|-0[.][0-9]{14}-|-[0-9A-Za-z.-]+[.]0[.][0-9]{14}-)[0-9a-f]{12}([+][0-9A-Za-z.-]+)?$`)

// BuildInfo is the public version payload returned by the CLI.
type BuildInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

// NewBuildInfo returns the CLI version payload. Explicit linker-provided values
// win, but module-aware installs such as `go install module@v0.1.0` can still
// surface the tagged module version without running this repo's Makefile.
func NewBuildInfo(name string, version string, commit string, buildDate string) BuildInfo {
	info := BuildInfo{
		Name:      name,
		Version:   normalizeModuleVersion(version),
		Commit:    commit,
		BuildDate: buildDate,
	}

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}

	if shouldUseModuleVersion(info.Version) {
		if moduleVersion := releaseModuleVersion(buildInfo.Main.Version); moduleVersion != "" {
			info.Version = moduleVersion
		}
	}

	if !hasValue(info.Commit) {
		if revision := buildSetting(buildInfo, "vcs.revision"); revision != "" {
			info.Commit = shortRevision(revision)
		}
	}
	if !hasValue(info.BuildDate) {
		if vcsTime := buildSetting(buildInfo, "vcs.time"); vcsTime != "" {
			info.BuildDate = vcsTime
		}
	}

	return info
}

func shouldUseModuleVersion(version string) bool {
	return version == "" || version == devVersion || version == "(devel)"
}

func releaseModuleVersion(version string) string {
	if version == "" || version == "(devel)" || pseudoModuleVersionPattern.MatchString(version) {
		return ""
	}
	return normalizeModuleVersion(version)
}

func normalizeModuleVersion(version string) string {
	if strings.HasPrefix(version, "v") && len(version) > 1 && version[1] >= '0' && version[1] <= '9' {
		return strings.TrimPrefix(version, "v")
	}
	return version
}

func buildSetting(info *debug.BuildInfo, key string) string {
	for _, setting := range info.Settings {
		if setting.Key == key {
			return setting.Value
		}
	}
	return ""
}

func shortRevision(revision string) string {
	if len(revision) <= 12 {
		return revision
	}
	return revision[:12]
}

func hasValue(value string) bool {
	return strings.TrimSpace(value) != "" && value != "unknown"
}
