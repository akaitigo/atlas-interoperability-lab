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
	if report.WritesPerformed || report.MigrationState != "requires-depth-parity-and-certificate-renewal" || report.RequiredDepthAxes != 18 || report.RequiredSurfacePatternRows != 850 || report.OpenSurfacePatternGaps != 421 || report.CoreEvidenceDependencyCommit != evidenceDependencyCoreCommit || len(report.Subjects) != 2 || !contains(report.Warnings, "surface-pattern-proof-closure-required") || !contains(report.Warnings, "evidence-dependency-consumer-matrix-required") {
		t.Fatalf("unexpected migration report: %#v", report)
	}
}

func TestEvidenceDependencyConsumerCompatibilityMatrix(t *testing.T) {
	report, err := RunEvidenceDependencyConsumerMatrix("../..", "tests/fixtures/evidence-dependency-consumer.matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "pass" || report.CoreCommit != evidenceDependencyCoreCommit || report.CoreStatus != "main-ci-confirmed" || report.Thresholds.SubjectCounts != "subject-defined" || report.Thresholds.SubjectProfiles != "subject-defined" || !sameSet(report.Thresholds.Predicates, []string{"transitive-staleness", "actual-rerun", "complete-output-closure", "proof-structure-invariant", "closure-plan-structure-invariant"}) || len(report.Results) != 21 {
		t.Fatalf("unexpected Evidence Dependency Matrix: %#v", report)
	}
	byCase := map[string]map[string]string{}
	for _, result := range report.Results {
		if result.Verdict != "pass" || !result.ConsumerIndependent {
			t.Fatalf("consumer別判定が不一致です: %#v", result)
		}
		if byCase[result.CaseID] == nil {
			byCase[result.CaseID] = map[string]string{}
		}
		byCase[result.CaseID][result.Consumer] = result.Classification + ":" + result.GateState + ":" + result.CertificateState
	}
	for id, states := range byCase {
		if len(states) != 3 || states["codex"] != states["claude-code"] || states["codex"] != states["generic-cli"] {
			t.Fatalf("consumer間でstale/closure判定が異なります: %s %#v", id, states)
		}
	}
}

func TestMultiSubjectCompositionCompatibilityDoesNotHideGaps(t *testing.T) {
	report, err := RunCompositionCompatibilityMatrix("../..", "tests/fixtures/composition-compatibility.matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "pass" || report.CoreCommit != evidenceDependencyCoreCommit || len(report.Results) != 24 {
		t.Fatalf("unexpected Composition Compatibility Matrix: %#v", report)
	}
	honestIncomplete, claimLinkRejections := 0, 0
	for _, result := range report.Results {
		if result.Verdict != "pass" || !result.ConsumerIndependent || result.DefinitiveEligible || len(result.Subjects) != 2 || !containsAll(result.InheritedGaps, []string{"core-v2-draft", "subject-depth-parity-incomplete", "subject-probe-atomic-binding-gap", "surface-pattern-proof-gaps"}) {
			t.Fatalf("Gapまたはconsumer判定が不正です: %#v", result)
		}
		if result.CompatibilityState == "incomplete" {
			honestIncomplete++
		}
		if result.CaseID == "reject-cross-subject-claim-link-gap" && result.ClaimGraphState == "reject" && len(result.FailedSubjects) == 0 {
			claimLinkRejections++
		}
		if len(result.FailedSubjects) == 1 {
			failed := result.FailedSubjects[0]
			for _, subject := range result.Subjects {
				if subject.Name == failed && (subject.GateState != "reject" || subject.CertificateState != "reject") {
					t.Fatalf("失敗Subjectが構成成功で隠されています: %#v", result)
				}
			}
		}
	}
	if honestIncomplete != 3 || claimLinkRejections != 3 {
		t.Fatalf("incompleteまたはClaim link negative fixtureのconsumer網羅が不足しています: incomplete=%d claim=%d", honestIncomplete, claimLinkRejections)
	}
}
