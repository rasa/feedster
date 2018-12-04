package version

import (
	"testing"
)

func TestVersion(t *testing.T) {
	if VERSION == "" {
		t.Error("Expected VERSION to be non-empty")
	}
	if GITCOMMIT == "" {
		t.Error("Expected GITCOMMIT to be non-empty")
	}
}
