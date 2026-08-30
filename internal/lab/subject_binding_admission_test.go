package lab

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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
	for _, candidate := range report.Candidates {
		if candidate.AdmissionState != "rejected" || len(candidate.RejectionReasons) != 7 {
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

func TestSubjectBindingEvidenceIsByteIdenticalAcrossLocalAndPublicBoundaries(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	t.Setenv("ATLAS_LAB_PUBLIC_CI_ATTESTATION", "")
	localReport, err := EvaluateSubjectBindingAdmission(root, root)
	if err != nil {
		t.Fatal(err)
	}
	localMatrix, err := RunSubjectBindingAdmissionMatrix(root, "tests/fixtures/subject-binding-admission.matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	localRoot := t.TempDir()
	if err := PersistSubjectBindingAdmissionEvidence(localRoot, localReport, localMatrix); err != nil {
		t.Fatal(err)
	}
	publicRoot := t.TempDir()
	if err := PersistSubjectBindingAdmissionEvidence(publicRoot, localReport, localMatrix); err != nil {
		t.Fatal(err)
	}
	trackedBefore := map[string][]byte{}
	for _, name := range []string{"subject-binding-admission.json", "subject-binding-admission.matrix.json"} {
		localData, err := os.ReadFile(filepath.Join(localRoot, "evidence", "preview", name))
		if err != nil {
			t.Fatal(err)
		}
		publicData, err := os.ReadFile(filepath.Join(publicRoot, "evidence", "preview", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(localData, publicData) {
			t.Fatalf("local/public Subject binding Evidenceがbyte-identicalではありません: %s", name)
		}
		trackedBefore[name] = append([]byte{}, publicData...)
		for _, forbidden := range []string{"generated_at", "verification_mode", "live_verified", localRoot, publicRoot} {
			if bytes.Contains(localData, []byte(forbidden)) {
				t.Fatalf("動的fieldまたはhost pathがEvidenceへ混入しました: %s", forbidden)
			}
		}
	}
	t.Setenv("ATLAS_LAB_PUBLIC_CI_ATTESTATION", "1")
	publicReport, err := EvaluateSubjectBindingAdmission(root, root)
	if err != nil {
		t.Fatal(err)
	}
	publicMatrix, err := RunSubjectBindingAdmissionMatrix(root, "tests/fixtures/subject-binding-admission.matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := PersistSubjectBindingAdmissionEvidence(publicRoot, publicReport, publicMatrix); err != nil {
		t.Fatal(err)
	}
	for name, before := range trackedBefore {
		after, err := os.ReadFile(filepath.Join(publicRoot, "evidence", "preview", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("public検証がtracked Subject binding Evidenceを書き換えました: %s", name)
		}
	}
	if !reflect.DeepEqual(localReport, publicReport) || !reflect.DeepEqual(localMatrix, publicMatrix) {
		t.Fatal("同じtracked inputsからlocal/publicで異なるSubject binding構造が生成されました")
	}
	driftPath := filepath.Join(publicRoot, "evidence", "preview", "subject-binding-admission.json")
	drifted := append([]byte{}, trackedBefore["subject-binding-admission.json"]...)
	drifted[len(drifted)-2] = ' '
	if err := os.WriteFile(driftPath, drifted, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := PersistSubjectBindingAdmissionEvidence(publicRoot, publicReport, publicMatrix); err == nil {
		t.Fatal("public境界でtracked Subject binding Evidenceのbyte driftが拒否されません")
	}
}

func TestSubjectBindingEvidenceRejectsOrderAndPromotionDriftAcrossCleanRoots(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	data, err := os.ReadFile(filepath.Join(root, subjectBindingCandidateLockPath))
	if err != nil {
		t.Fatal(err)
	}
	var lock SubjectBindingCandidateLock
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatal(err)
	}
	control := buildSubjectBindingAdmissionReport(lock)
	mutated := control
	mutated.Candidates = append([]SubjectBindingCandidateResult{}, control.Candidates...)
	mutated.Candidates[0], mutated.Candidates[1] = mutated.Candidates[1], mutated.Candidates[0]
	mutated.DefinitiveEligible = true
	if err := validateSubjectBindingAdmissionReport(mutated); err == nil {
		t.Fatal("Subject順序driftと未完成候補の昇格が拒否されません")
	}
	controlRoot, negativeRoot := t.TempDir(), t.TempDir()
	matrix, err := RunSubjectBindingAdmissionMatrix(root, "tests/fixtures/subject-binding-admission.matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := PersistSubjectBindingAdmissionEvidence(controlRoot, control, matrix); err != nil {
		t.Fatal(err)
	}
	if err := PersistSubjectBindingAdmissionEvidence(negativeRoot, mutated, matrix); err == nil {
		t.Fatal("negative rootへ不正なSubject binding Evidenceを保存できました")
	}
}
