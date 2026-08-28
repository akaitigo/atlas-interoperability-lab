package lab

import "testing"

func TestClassifyActionFailure(t *testing.T) {
	tests := []struct {
		message string
		want    string
	}{
		{"status expected 200 got 502", "oracle-status-mismatch"},
		{"json assertion failed", "oracle-data-mismatch"},
		{"connection timeout", "subject-unreachable"},
		{"compare value mismatch", "boundary-state-mismatch"},
		{"unknown failure", "oracle-mismatch"},
	}
	for _, test := range tests {
		got, _ := classifyActionFailure(test.message)
		if got != test.want {
			t.Fatalf("classifyActionFailure(%q)=%q, want %q", test.message, got, test.want)
		}
	}
}

func TestDiagnosePassingEvidence(t *testing.T) {
	diagnosis := Diagnose("../..", "local")
	if diagnosis.Verdict != "pass" || len(diagnosis.Findings) != 1 || diagnosis.Findings[0].Code != "run-healthy" {
		t.Fatalf("unexpected diagnosis: %#v", diagnosis)
	}
}
