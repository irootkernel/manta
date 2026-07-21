package cli

import "testing"

func TestNewBuildInfoPreservesExplicitReleaseMetadata(t *testing.T) {
	info := NewBuildInfo("manta", "1.2.3", "abc123", "2026-01-01T00:00:00Z")

	want := BuildInfo{Name: "manta", Version: "1.2.3", Commit: "abc123", BuildDate: "2026-01-01T00:00:00Z"}
	if info != want {
		t.Fatalf("info = %#v, want %#v", info, want)
	}
}

func TestNewBuildInfoNormalizesGoModuleVersionPrefix(t *testing.T) {
	info := NewBuildInfo("manta", "v1.2.3", "abc123", "2026-01-01T00:00:00Z")

	if info.Version != "1.2.3" {
		t.Fatalf("Version = %q, want 1.2.3", info.Version)
	}
}

func TestReleaseModuleVersionRejectsPseudoVersions(t *testing.T) {
	for _, version := range []string{"v0.0.0-20260504100202-d062baa42ee2", "v0.0.0-20260504100202-d062baa42ee2+dirty", "v1.2.4-0.20260504100202-d062baa42ee2", "v1.2.3-rc.1.0.20260504100202-d062baa42ee2"} {
		if got := releaseModuleVersion(version); got != "" {
			t.Fatalf("releaseModuleVersion(%q) = %q, want empty", version, got)
		}
	}
}

func TestReleaseModuleVersionAcceptsTaggedVersions(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{in: "v0.1.0", want: "0.1.0"},
		{in: "v0.1.0-rc.1", want: "0.1.0-rc.1"},
	} {
		if got := releaseModuleVersion(tc.in); got != tc.want {
			t.Fatalf("releaseModuleVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
