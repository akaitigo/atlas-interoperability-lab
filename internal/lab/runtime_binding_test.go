package lab

import (
	"path/filepath"
	"testing"
)

func TestSavedRuntimeBindingEvidence(t *testing.T) {
	root := "../.."
	for _, profile := range []string{"local", "container"} {
		profile := profile
		t.Run(profile, func(t *testing.T) {
			var evidence RuntimeBindingEvidence
			path := filepath.Join(root, "evidence", "preview", "runtime-binding", profile+".binding.json")
			if err := LoadJSON(path, &evidence); err != nil {
				t.Fatal(err)
			}
			if err := ValidateRuntimeBindingEvidence(root, evidence); err != nil {
				t.Fatal(err)
			}
			if evidence.DefinitiveEligible || len(evidence.Subjects) != 2 || len(evidence.Executable.Attestations) != 2 || evidence.RuntimeEvidence.ScenarioCount != 5 || !sameSet(evidence.Gaps, []string{"subject-v2-certificate-atomic-binding-unavailable"}) {
				t.Fatalf("Runtime Bindingが不足を正直に保持していません: %#v", evidence)
			}
		})
	}
}

func TestRuntimeBindingEvidenceRejectsPromotionAndClosureShrink(t *testing.T) {
	root := "../.."
	var baseline RuntimeBindingEvidence
	if err := LoadJSON(filepath.Join(root, "evidence", "preview", "runtime-binding", "local.binding.json"), &baseline); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*RuntimeBindingEvidence)
	}{
		{name: "definitive-promotion", mutate: func(value *RuntimeBindingEvidence) { value.DefinitiveEligible = true }},
		{name: "certificate-gap-withdrawal", mutate: func(value *RuntimeBindingEvidence) { value.Gaps = nil }},
		{name: "executable-attestation-withdrawal", mutate: func(value *RuntimeBindingEvidence) { value.Executable.Attestations = value.Executable.Attestations[:1] }},
		{name: "executable-attestation-digest-drift", mutate: func(value *RuntimeBindingEvidence) {
			value.Executable.Attestations[0].ObservedDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{name: "scenario-withdrawal", mutate: func(value *RuntimeBindingEvidence) {
			value.RuntimeEvidence.Scenarios = value.RuntimeEvidence.Scenarios[:4]
		}},
		{name: "release-lock-drift", mutate: func(value *RuntimeBindingEvidence) {
			value.Subjects[0].ReleaseDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{name: "runtime-digest-invalid", mutate: func(value *RuntimeBindingEvidence) { value.Executable.RuntimeBinaryDigest = "sha256:invalid" }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			candidate := cloneRuntimeBindingEvidence(baseline)
			testCase.mutate(&candidate)
			if err := ValidateRuntimeBindingEvidence(root, candidate); err == nil {
				t.Fatal("不正なRuntime Binding Evidenceが受理されました")
			}
		})
	}
}

func cloneRuntimeBindingEvidence(source RuntimeBindingEvidence) RuntimeBindingEvidence {
	clone := source
	clone.Gaps = append([]string{}, source.Gaps...)
	clone.Subjects = append([]RuntimeSubjectBinding{}, source.Subjects...)
	clone.Executable.Attestations = append([]RuntimeExecutableAttestation{}, source.Executable.Attestations...)
	clone.RuntimeEvidence.Scenarios = append([]PreviewArtifactLock{}, source.RuntimeEvidence.Scenarios...)
	return clone
}
