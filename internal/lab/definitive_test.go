package lab

import "testing"

func TestDefinitiveGateV2FixtureMatrix(t *testing.T) {
	report, err := RunDefinitiveMatrix("../..", "tests/fixtures/definitive-gate-v2.matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "pass" || len(report.Cases) != 10 {
		t.Fatalf("unexpected matrix report: %#v", report)
	}
	for _, testCase := range report.Cases {
		if testCase.EffectiveState == "definitive-complete" {
			t.Fatalf("draft Core v2 must never emit definitive-complete: %s", testCase.ID)
		}
	}
}

func TestV1MigrationIsNonDestructive(t *testing.T) {
	report, err := PlanV2Migration("../..", "compositions/fixture-stage2.json")
	if err != nil {
		t.Fatal(err)
	}
	if report.WritesPerformed || report.MigrationState != "requires-certificate-renewal" || len(report.Subjects) != 2 {
		t.Fatalf("unexpected migration report: %#v", report)
	}
}
