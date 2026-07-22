package safety

import (
	"bytes"
	"strings"
	"testing"
)

func TestDecodeYAMLStrictRejectsOversizedInputBeforeDecode(t *testing.T) {
	t.Parallel()
	data := append([]byte("value: "), bytes.Repeat([]byte("x"), MaxConfigRuleInputBytes)...)
	var target struct {
		Value string `yaml:"value"`
	}
	err := DecodeYAMLStrict(data, &target)
	if err == nil || !strings.Contains(err.Error(), "YAML input exceeds 262144 bytes") {
		t.Fatalf("expected YAML size error, got %v", err)
	}
	if target.Value != "" {
		t.Fatalf("oversized YAML was decoded: value length=%d", len(target.Value))
	}
}
