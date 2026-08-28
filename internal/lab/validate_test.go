package lab

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPreflightAcceptsCompletedPinnedReleases(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	validated, err := Preflight(root, "compositions/fixture-stage2.json", "local")
	if err != nil {
		t.Fatal(err)
	}
	if validated.Manifest.Stage != 2 || len(validated.Releases) != 2 {
		t.Fatalf("unexpected validated composition: %#v", validated)
	}
}

func TestPreflightRejectsIncompleteSubject(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	_, err := Preflight(root, "tests/fixtures/composition-incomplete.json", "local")
	if err == nil || !strings.Contains(err.Error(), "未完成Release") {
		t.Fatalf("expected incomplete release rejection, got %v", err)
	}
}

func TestRequiredAxesAreClosed(t *testing.T) {
	if len(RequiredAxes) != 10 {
		t.Fatalf("expected 10 required axes, got %d", len(RequiredAxes))
	}
	if !sameSet(RequiredAxes, []string{"communication", "identity", "data", "messaging", "deployment", "observability", "security-boundary", "failure-propagation", "recovery", "compatibility"}) {
		t.Fatal("required axes drifted")
	}
}
