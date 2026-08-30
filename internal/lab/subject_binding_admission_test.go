package lab

import (
	"os"
	"testing"
)

func TestActualSubjectBindingAdmissionRejectsV1OnlyCandidates(t *testing.T) {
	report, err := EvaluateSubjectBindingAdmission("../..", "../..")
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "pass" || report.CompletionState != "incomplete" || report.DefinitiveEligible || len(report.Candidates) != 3 || report.NegativeCases != 14 || !sameSet(report.Gaps, []string{"subject-v2-certificate-atomic-binding-unavailable"}) {
		t.Fatalf("Actual Subject binding admissionが不正です: %#v", report)
	}
	publicMode := os.Getenv("ATLAS_LAB_PUBLIC_CI_ATTESTATION") == "1"
	for _, candidate := range report.Candidates {
		if candidate.AdmissionState != "rejected" || len(candidate.RejectionReasons) != 7 || candidate.LiveVerified == publicMode {
			t.Fatalf("未完成Actual Subjectの個別拒否が不正です: %#v", candidate)
		}
	}
}

func TestActualSubjectBindingAdmissionNegativeMatrix(t *testing.T) {
	report, err := RunSubjectBindingAdmissionMatrix("../..", "tests/fixtures/subject-binding-admission.matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "pass" || len(report.Results) != 14 {
		t.Fatalf("Subject binding admission negative matrixが不正です: %#v", report)
	}
	for _, result := range report.Results {
		if !result.Rejected || result.Verdict != "pass" {
			t.Fatalf("未完成Subject promotion mutationが拒否されません: %#v", result)
		}
	}
}
