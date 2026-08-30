package lab

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestSavedCompositionEvidenceDependencyClosure(t *testing.T) {
	audit, err := AuditCompositionEvidenceDependency("../..", compositionEvidenceGraphPath)
	if err != nil {
		t.Fatal(err)
	}
	if audit.Verdict != "pass" || audit.CoreCommit != evidenceDependencyCoreCommit || audit.Inputs != 6 || audit.Outputs != 21 || audit.Runs != 3 || audit.ChangedInputs != 0 || audit.AffectedOutputs != 0 || audit.CompletionState != "incomplete" || audit.DefinitiveEligible || !sameSet(audit.Gaps, []string{"subject-depth-parity-incomplete", "subject-v2-certificate-atomic-binding-unavailable", "surface-pattern-proof-gaps"}) {
		t.Fatalf("Composition Evidence closureが不正です: %#v", audit)
	}
}

func TestCompositionEvidenceDependencyNegativeMatrix(t *testing.T) {
	report, err := RunCompositionEvidenceDependencyMatrix("../..", "tests/fixtures/composition-evidence-dependency.matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "pass" || report.CoreCommit != evidenceDependencyCoreCommit || len(report.Results) != 11 {
		t.Fatalf("Composition Evidence negative matrixが不正です: %#v", report)
	}
	rejections := 0
	for _, result := range report.Results {
		if result.Verdict != "pass" {
			t.Fatalf("negative caseがfailです: %#v", result)
		}
		if result.State == "reject" {
			rejections++
		}
	}
	if rejections != 10 {
		t.Fatalf("negative rejection数が不正です: %d", rejections)
	}
}

func TestCompositionEvidenceDocumentationMatchesCurrentClosure(t *testing.T) {
	audit, err := AuditCompositionEvidenceDependency("../..", compositionEvidenceGraphPath)
	if err != nil {
		t.Fatal(err)
	}
	document, err := os.ReadFile("../../docs/COMPOSITION_EVIDENCE_DEPENDENCY.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(document)
	required := []string{
		fmt.Sprintf("%d件", audit.Outputs),
		"Actual Subject Admission／Negative Matrix",
		"process executableの独立attestationは実process／container観測とMigration Evidenceにより閉鎖済み",
	}
	required = append(required, audit.Gaps...)
	for _, fragment := range required {
		if !strings.Contains(text, fragment) {
			t.Fatalf("Composition Evidence文書が現行closureを記録していません: %s", fragment)
		}
	}
	for _, stale := range []string{"Closure Planの19件", "- process executableの独立attestation"} {
		if strings.Contains(text, stale) {
			t.Fatalf("Composition Evidence文書に閉鎖前の記述が残っています: %s", stale)
		}
	}
}
