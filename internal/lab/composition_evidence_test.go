package lab

import "testing"

func TestSavedCompositionEvidenceDependencyClosure(t *testing.T) {
	audit, err := AuditCompositionEvidenceDependency("../..", compositionEvidenceGraphPath)
	if err != nil {
		t.Fatal(err)
	}
	if audit.Verdict != "pass" || audit.CoreCommit != evidenceDependencyCoreCommit || audit.Inputs != 6 || audit.Outputs != 19 || audit.Runs != 3 || audit.ChangedInputs != 0 || audit.AffectedOutputs != 0 || audit.CompletionState != "incomplete" || audit.DefinitiveEligible || !sameSet(audit.Gaps, []string{"subject-depth-parity-incomplete", "subject-v2-certificate-atomic-binding-unavailable", "surface-pattern-proof-gaps"}) {
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
