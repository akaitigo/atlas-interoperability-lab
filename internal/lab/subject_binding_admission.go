package lab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const subjectBindingCandidateLockPath = "compatibility/subject-binding-candidates.lock.json"

type SubjectBindingVerificationBoundary struct {
	LocalGitObjectVerification string `json:"local_git_object_verification"`
	PublicCIVerification       string `json:"public_ci_verification"`
	LocalGateRequired          bool   `json:"local_gate_required"`
	CompletionEffect           string `json:"completion_effect"`
}

type SubjectBindingCertificate struct {
	Path              string `json:"path"`
	Digest            string `json:"digest"`
	SchemaVersion     int    `json:"schema_version"`
	CorePolicyVersion string `json:"core_policy_version"`
	Release           string `json:"release"`
	SubjectCommit     string `json:"subject_commit"`
	SignatureKind     string `json:"signature_kind"`
}

type SubjectBindingRequirements struct {
	FixedReleaseDigest             bool `json:"fixed_release_digest"`
	V2Certificate                  bool `json:"v2_certificate"`
	CertificateCommitAtomic        bool `json:"certificate_commit_atomic"`
	CryptographicSignatureVerified bool `json:"cryptographic_signature_verified"`
	DepthComplete                  bool `json:"depth_complete"`
	SurfacePatternComplete         bool `json:"surface_pattern_complete"`
	AuthorityAtomicBinding         bool `json:"authority_atomic_binding"`
}

type SubjectBindingCandidate struct {
	SubjectID        string                     `json:"subject_id"`
	Repository       string                     `json:"repository"`
	Commit           string                     `json:"commit"`
	Certificate      SubjectBindingCertificate  `json:"certificate"`
	Requirements     SubjectBindingRequirements `json:"requirements"`
	AdmissionState   string                     `json:"admission_state"`
	RejectionReasons []string                   `json:"rejection_reasons"`
}

type SubjectBindingCandidateLock struct {
	SchemaVersion        int                                `json:"schema_version"`
	ID                   string                             `json:"id"`
	CoreCommit           string                             `json:"core_commit"`
	Policy               string                             `json:"policy"`
	VerificationBoundary SubjectBindingVerificationBoundary `json:"verification_boundary"`
	Candidates           []SubjectBindingCandidate          `json:"candidates"`
	CompletionState      string                             `json:"completion_state"`
	Gaps                 []string                           `json:"gaps"`
	DefinitiveEligible   bool                               `json:"definitive_eligible"`
}

type SubjectBindingCandidateResult struct {
	SubjectID         string   `json:"subject_id"`
	Commit            string   `json:"commit"`
	CertificateDigest string   `json:"certificate_digest"`
	AdmissionState    string   `json:"admission_state"`
	RejectionReasons  []string `json:"rejection_reasons"`
}

type SubjectBindingAdmissionReport struct {
	SchemaVersion        int                                `json:"schema_version"`
	Gate                 string                             `json:"gate"`
	CoreCommit           string                             `json:"core_commit"`
	VerificationBoundary SubjectBindingVerificationBoundary `json:"verification_boundary"`
	Candidates           []SubjectBindingCandidateResult    `json:"candidates"`
	CompletionState      string                             `json:"completion_state"`
	Gaps                 []string                           `json:"gaps"`
	DefinitiveEligible   bool                               `json:"definitive_eligible"`
	NegativeCases        int                                `json:"negative_cases"`
	Verdict              string                             `json:"verdict"`
}

type SubjectBindingAdmissionMatrix struct {
	SchemaVersion int                                 `json:"schema_version"`
	Cases         []SubjectBindingAdmissionMatrixCase `json:"cases"`
}

type SubjectBindingAdmissionMatrixCase struct {
	ID        string `json:"id"`
	Operation string `json:"operation"`
}

type SubjectBindingAdmissionMatrixResult struct {
	SchemaVersion int                                       `json:"schema_version"`
	Results       []SubjectBindingAdmissionMatrixCaseResult `json:"results"`
	Verdict       string                                    `json:"verdict"`
}

type SubjectBindingAdmissionMatrixCaseResult struct {
	ID       string `json:"id"`
	Rejected bool   `json:"rejected"`
	Verdict  string `json:"verdict"`
}

func EvaluateSubjectBindingAdmission(root, referenceRoot string) (SubjectBindingAdmissionReport, error) {
	data, err := os.ReadFile(resolve(root, subjectBindingCandidateLockPath))
	if err != nil {
		return SubjectBindingAdmissionReport{}, err
	}
	var lock SubjectBindingCandidateLock
	if err := decodeStrictJSON(subjectBindingCandidateLockPath, data, &lock); err != nil {
		return SubjectBindingAdmissionReport{}, err
	}
	if err := validateSubjectBindingLock(lock); err != nil {
		return SubjectBindingAdmissionReport{}, err
	}
	publicMode := os.Getenv("ATLAS_LAB_PUBLIC_CI_ATTESTATION") == "1"
	if publicMode {
		reader := func(path string) ([]byte, error) { return os.ReadFile(resolve(referenceRoot, path)) }
		if _, err := validateFEPublicAttestation(reader); err != nil {
			return SubjectBindingAdmissionReport{}, fmt.Errorf("Subject binding public境界のowner署名を検証できません: %w", err)
		}
	}
	report := buildSubjectBindingAdmissionReport(lock)
	for _, candidate := range lock.Candidates {
		if !publicMode {
			if err := verifyLiveSubjectCandidate(referenceRoot, candidate); err != nil {
				return report, err
			}
		}
	}
	return report, nil
}

func PersistSubjectBindingAdmissionEvidence(root string, report SubjectBindingAdmissionReport, matrix SubjectBindingAdmissionMatrixResult) error {
	if err := validateSubjectBindingAdmissionReport(report); err != nil {
		return err
	}
	if err := validateSubjectBindingAdmissionMatrixResult(matrix); err != nil {
		return err
	}
	outputs := []struct {
		path  string
		value any
	}{
		{path: "evidence/preview/subject-binding-admission.json", value: report},
		{path: "evidence/preview/subject-binding-admission.matrix.json", value: matrix},
	}
	if os.Getenv("ATLAS_LAB_PUBLIC_CI_ATTESTATION") == "1" {
		return validateTrackedSubjectBindingEvidence(root, outputs)
	}
	if err := WriteJSON(filepath.Join(root, "evidence", "preview", "subject-binding-admission.json"), report); err != nil {
		return err
	}
	return WriteJSON(filepath.Join(root, "evidence", "preview", "subject-binding-admission.matrix.json"), matrix)
}

func validateTrackedSubjectBindingEvidence(root string, outputs []struct {
	path  string
	value any
}) error {
	for _, output := range outputs {
		expected, err := subjectBindingEvidenceBytes(output.value)
		if err != nil {
			return err
		}
		tracked, err := os.ReadFile(resolve(root, output.path))
		if err != nil {
			return fmt.Errorf("public CIでtracked Subject binding Evidenceを読めません: %s: %w", output.path, err)
		}
		if !bytes.Equal(tracked, expected) {
			return fmt.Errorf("public CIでtracked Subject binding Evidenceが決定論的出力と一致しません: %s", output.path)
		}
	}
	return nil
}

func subjectBindingEvidenceBytes(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func buildSubjectBindingAdmissionReport(lock SubjectBindingCandidateLock) SubjectBindingAdmissionReport {
	report := SubjectBindingAdmissionReport{SchemaVersion: 1, Gate: "actual-subject-binding-admission", CoreCommit: evidenceDependencyCoreCommit, VerificationBoundary: lock.VerificationBoundary, Candidates: []SubjectBindingCandidateResult{}, CompletionState: "incomplete", Gaps: append([]string{}, lock.Gaps...), DefinitiveEligible: false, NegativeCases: 14, Verdict: "pass"}
	for _, candidate := range lock.Candidates {
		report.Candidates = append(report.Candidates, SubjectBindingCandidateResult{SubjectID: candidate.SubjectID, Commit: candidate.Commit, CertificateDigest: candidate.Certificate.Digest, AdmissionState: candidate.AdmissionState, RejectionReasons: append([]string{}, candidate.RejectionReasons...)})
	}
	return report
}

func validateSubjectBindingAdmissionReport(report SubjectBindingAdmissionReport) error {
	expectedBoundary := SubjectBindingVerificationBoundary{LocalGitObjectVerification: "required", PublicCIVerification: "owner-signed-tracked-input", LocalGateRequired: true, CompletionEffect: "none"}
	if report.SchemaVersion != 1 || report.Gate != "actual-subject-binding-admission" || report.CoreCommit != evidenceDependencyCoreCommit || report.VerificationBoundary != expectedBoundary || report.CompletionState != "incomplete" || report.DefinitiveEligible || report.NegativeCases != 14 || report.Verdict != "pass" || !sameSet(report.Gaps, []string{"subject-v2-certificate-atomic-binding-unavailable"}) {
		return fmt.Errorf("Actual Subject binding Evidence境界が不正です")
	}
	expectedOrder := []string{"rabbitmq-reference-atlas", "postgresql-reference-atlas", "zero-trust-reference-atlas"}
	expected := expectedSubjectBindingCandidates()
	if len(report.Candidates) != len(expectedOrder) {
		return fmt.Errorf("Actual Subject binding Evidence分母が不正です")
	}
	for index, candidate := range report.Candidates {
		if candidate.SubjectID != expectedOrder[index] {
			return fmt.Errorf("Actual Subject binding Evidence順序が不正です")
		}
		want := expected[candidate.SubjectID]
		if candidate.Commit != want.Commit || candidate.CertificateDigest != want.Certificate.Digest || candidate.AdmissionState != "rejected" || !sameSet(candidate.RejectionReasons, want.RejectionReasons) {
			return fmt.Errorf("Actual Subject binding EvidenceがLockと一致しません: %s", candidate.SubjectID)
		}
	}
	return nil
}

func validateSubjectBindingLock(lock SubjectBindingCandidateLock) error {
	expectedBoundary := SubjectBindingVerificationBoundary{LocalGitObjectVerification: "required", PublicCIVerification: "owner-signed-tracked-input", LocalGateRequired: true, CompletionEffect: "none"}
	if lock.SchemaVersion != 1 || lock.ID != "actual-subject-binding-candidates-v1" || lock.CoreCommit != evidenceDependencyCoreCommit || lock.Policy != "per-subject-no-aggregation" || lock.VerificationBoundary != expectedBoundary || lock.CompletionState != "incomplete" || lock.DefinitiveEligible || !sameSet(lock.Gaps, []string{"subject-v2-certificate-atomic-binding-unavailable"}) {
		return fmt.Errorf("Actual Subject binding admission境界が不正です")
	}
	expected := expectedSubjectBindingCandidates()
	if len(lock.Candidates) != len(expected) {
		return fmt.Errorf("Actual Subject candidate分母が縮小されています")
	}
	seen := map[string]bool{}
	expectedOrder := []string{"rabbitmq-reference-atlas", "postgresql-reference-atlas", "zero-trust-reference-atlas"}
	for index, candidate := range lock.Candidates {
		if candidate.SubjectID != expectedOrder[index] {
			return fmt.Errorf("Actual Subject candidate順序が不正です")
		}
		want, ok := expected[candidate.SubjectID]
		if !ok || seen[candidate.SubjectID] {
			return fmt.Errorf("Actual Subject candidate identityが不正です: %s", candidate.SubjectID)
		}
		seen[candidate.SubjectID] = true
		if candidate.Repository != want.Repository || candidate.Commit != want.Commit || candidate.Certificate != want.Certificate || candidate.Requirements != (SubjectBindingRequirements{}) || candidate.AdmissionState != "rejected" || !sameSet(candidate.RejectionReasons, want.RejectionReasons) {
			return fmt.Errorf("未完成Actual Subjectの拒否境界が変更されています: %s", candidate.SubjectID)
		}
	}
	return nil
}

func expectedSubjectBindingCandidates() map[string]SubjectBindingCandidate {
	common := []string{"fixed-release-digest-unavailable", "certificate-v1-only", "cryptographic-signature-missing", "v2-depth-unavailable", "surface-pattern-proof-unavailable", "atomic-authority-binding-unavailable"}
	return map[string]SubjectBindingCandidate{
		"rabbitmq-reference-atlas": {
			SubjectID: "rabbitmq-reference-atlas", Repository: "https://github.com/akaitigo/rabbitmq-reference-atlas", Commit: "22ab07cc6c3d92ab489fe6ff8855c9fb8a97db5a",
			Certificate:      SubjectBindingCertificate{Path: "evidence/completion-certificate.json", Digest: "sha256:8dbf8e2820e0e839eef7574e447f5607350f84f7a6367af58022d34a6ce69099", SchemaVersion: 1, CorePolicyVersion: "unasserted", Release: "unasserted", SubjectCommit: "unasserted", SignatureKind: "none"},
			RejectionReasons: append(append([]string{}, common...), "certificate-commit-unbound"),
		},
		"postgresql-reference-atlas": {
			SubjectID: "postgresql-reference-atlas", Repository: "https://github.com/akaitigo/postgresql-reference-atlas", Commit: "8a4259d2de288178b8b87f09a09e5b57654c88e0",
			Certificate:      SubjectBindingCertificate{Path: "evidence/completion-certificate.json", Digest: "sha256:7f88fc0e072438d84b296b51428e88983688eca85cf4f97d7bf819b0bde9a26a", SchemaVersion: 1, CorePolicyVersion: "1.0.0", Release: "v1.0.0", SubjectCommit: "9704d11b19a65755cb6d5738131d104d48309ef5", SignatureKind: "payload-sha256"},
			RejectionReasons: append(append([]string{}, common...), "certificate-main-commit-mismatch"),
		},
		"zero-trust-reference-atlas": {
			SubjectID: "zero-trust-reference-atlas", Repository: "https://github.com/akaitigo/zero-trust-reference-atlas", Commit: "fef700605d4aecb7d8f7975a6a4067bfe390ac86",
			Certificate:      SubjectBindingCertificate{Path: "evidence/completion-certificate.json", Digest: "sha256:59f0e0b2f8a85f3705fd9b4d5198f2df6f662932ca5a8f616091650cb0e3f700", SchemaVersion: 1, CorePolicyVersion: "1.0.0", Release: "v1.0.0", SubjectCommit: "8d4ace98787167df25725756d4fc3ac22ba7d272", SignatureKind: "payload-sha256"},
			RejectionReasons: append(append([]string{}, common...), "certificate-main-commit-mismatch"),
		},
	}
}

func verifyLiveSubjectCandidate(referenceRoot string, candidate SubjectBindingCandidate) error {
	absoluteReferenceRoot, err := filepath.Abs(referenceRoot)
	if err != nil {
		return err
	}
	workspaceRoot := filepath.Dir(absoluteReferenceRoot)
	if configured := os.Getenv("ATLAS_LAB_SUBJECT_WORKSPACE_ROOT"); configured != "" {
		workspaceRoot, err = filepath.Abs(configured)
		if err != nil {
			return err
		}
	}
	repositoryName := strings.TrimPrefix(candidate.Repository, "https://github.com/akaitigo/")
	if repositoryName == candidate.Repository || strings.Contains(repositoryName, "/") || repositoryName == "" {
		return fmt.Errorf("Actual Subject repository identityが不正です: %s", candidate.SubjectID)
	}
	repositoryRoot := filepath.Join(workspaceRoot, repositoryName)
	if output, err := exec.Command("git", "-C", repositoryRoot, "cat-file", "-e", candidate.Commit+"^{commit}").CombinedOutput(); err != nil {
		return fmt.Errorf("Actual Subject固定Commitを検証できません: %s: %w: %s", candidate.SubjectID, err, strings.TrimSpace(string(output)))
	}
	data, err := exec.Command("git", "-C", repositoryRoot, "show", candidate.Commit+":"+candidate.Certificate.Path).Output()
	if err != nil {
		return fmt.Errorf("Actual Subject Certificate Git objectを読めません: %s: %w", candidate.SubjectID, err)
	}
	if DigestBytes(data) != candidate.Certificate.Digest {
		return fmt.Errorf("Actual Subject Certificate digestが固定値と一致しません: %s", candidate.SubjectID)
	}
	var observed struct {
		SchemaVersion     int             `json:"schema_version"`
		AtlasID           string          `json:"atlas_id"`
		AtlasRelease      string          `json:"atlas_release"`
		Commit            string          `json:"commit"`
		CorePolicyVersion string          `json:"core_policy_version"`
		Signature         json.RawMessage `json:"signature"`
	}
	if err := json.Unmarshal(data, &observed); err != nil {
		return err
	}
	signatureKind := "none"
	if len(observed.Signature) > 0 && !bytes.Equal(bytes.TrimSpace(observed.Signature), []byte("null")) {
		var signature struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(observed.Signature, &signature); err != nil {
			return err
		}
		signatureKind = signature.Type
	}
	corePolicy, release, subjectCommit := observed.CorePolicyVersion, observed.AtlasRelease, observed.Commit
	if corePolicy == "" {
		corePolicy = "unasserted"
	}
	if release == "" {
		release = "unasserted"
	}
	if subjectCommit == "" {
		subjectCommit = "unasserted"
	}
	if observed.SchemaVersion != candidate.Certificate.SchemaVersion || observed.AtlasID != candidate.SubjectID || corePolicy != candidate.Certificate.CorePolicyVersion || release != candidate.Certificate.Release || subjectCommit != candidate.Certificate.SubjectCommit || signatureKind != candidate.Certificate.SignatureKind {
		return fmt.Errorf("Actual Subject Certificate観測値がLockと一致しません: %s", candidate.SubjectID)
	}
	return nil
}

func RunSubjectBindingAdmissionMatrix(root, matrixPath string) (SubjectBindingAdmissionMatrixResult, error) {
	var matrix SubjectBindingAdmissionMatrix
	if err := LoadJSON(resolve(root, matrixPath), &matrix); err != nil {
		return SubjectBindingAdmissionMatrixResult{}, err
	}
	expectedCases := expectedSubjectBindingAdmissionMatrixCases()
	if matrix.SchemaVersion != 1 || len(matrix.Cases) != len(expectedCases) {
		return SubjectBindingAdmissionMatrixResult{}, fmt.Errorf("Subject binding admission negative matrix契約が不正です")
	}
	for index, testCase := range matrix.Cases {
		if testCase != expectedCases[index] {
			return SubjectBindingAdmissionMatrixResult{}, fmt.Errorf("Subject binding admission negative matrix順序が不正です")
		}
	}
	data, err := os.ReadFile(resolve(root, subjectBindingCandidateLockPath))
	if err != nil {
		return SubjectBindingAdmissionMatrixResult{}, err
	}
	var control SubjectBindingCandidateLock
	if err := json.Unmarshal(data, &control); err != nil {
		return SubjectBindingAdmissionMatrixResult{}, err
	}
	if err := validateSubjectBindingLock(control); err != nil {
		return SubjectBindingAdmissionMatrixResult{}, err
	}
	report := SubjectBindingAdmissionMatrixResult{SchemaVersion: 1, Results: []SubjectBindingAdmissionMatrixCaseResult{}, Verdict: "pass"}
	for _, testCase := range matrix.Cases {
		cloneData, _ := json.Marshal(control)
		var candidate SubjectBindingCandidateLock
		_ = json.Unmarshal(cloneData, &candidate)
		if err := mutateSubjectBindingCandidate(&candidate, testCase.Operation); err != nil {
			return report, err
		}
		rejected := validateSubjectBindingLock(candidate) != nil
		verdict := "pass"
		if !rejected {
			verdict = "fail"
			report.Verdict = "fail"
		}
		report.Results = append(report.Results, SubjectBindingAdmissionMatrixCaseResult{ID: testCase.ID, Rejected: rejected, Verdict: verdict})
	}
	if report.Verdict != "pass" {
		return report, fmt.Errorf("Subject binding admission negative matrixがfailです")
	}
	return report, nil
}

func expectedSubjectBindingAdmissionMatrixCases() []SubjectBindingAdmissionMatrixCase {
	return []SubjectBindingAdmissionMatrixCase{
		{ID: "reject.v1-certificate-promotion", Operation: "admit-v1-certificate"},
		{ID: "reject.v2-certificate-claim", Operation: "claim-v2-certificate"},
		{ID: "reject.release-digest-invention", Operation: "invent-release-digest"},
		{ID: "reject.signature-claim", Operation: "claim-cryptographic-signature"},
		{ID: "reject.commit-binding-claim", Operation: "claim-certificate-main-binding"},
		{ID: "reject.depth-completion-claim", Operation: "claim-depth-complete"},
		{ID: "reject.surface-pattern-completion-claim", Operation: "claim-surface-pattern-complete"},
		{ID: "reject.atomic-authority-claim", Operation: "claim-atomic-authority"},
		{ID: "reject.multi-subject-gap-aggregation", Operation: "aggregate-rejections"},
		{ID: "reject.candidate-denominator-shrink", Operation: "drop-candidate"},
		{ID: "reject.fixture-substitution", Operation: "substitute-fixture"},
		{ID: "reject.public-completion-effect", Operation: "public-completion-effect"},
		{ID: "reject.source-commit-drift", Operation: "source-commit-drift"},
		{ID: "reject.certificate-digest-drift", Operation: "certificate-digest-drift"},
	}
}

func validateSubjectBindingAdmissionMatrixResult(report SubjectBindingAdmissionMatrixResult) error {
	expected := expectedSubjectBindingAdmissionMatrixCases()
	if report.SchemaVersion != 1 || report.Verdict != "pass" || len(report.Results) != len(expected) {
		return fmt.Errorf("Subject binding admission negative Evidenceが不正です")
	}
	for index, result := range report.Results {
		if result.ID != expected[index].ID || !result.Rejected || result.Verdict != "pass" {
			return fmt.Errorf("Subject binding admission negative Evidence順序または拒否が不正です")
		}
	}
	return nil
}

func mutateSubjectBindingCandidate(lock *SubjectBindingCandidateLock, operation string) error {
	first := &lock.Candidates[0]
	switch operation {
	case "admit-v1-certificate":
		first.AdmissionState = "admitted"
	case "claim-v2-certificate":
		first.Certificate.SchemaVersion = 2
		first.Requirements.V2Certificate = true
	case "invent-release-digest":
		first.Requirements.FixedReleaseDigest = true
	case "claim-cryptographic-signature":
		first.Requirements.CryptographicSignatureVerified = true
	case "claim-certificate-main-binding":
		first.Requirements.CertificateCommitAtomic = true
	case "claim-depth-complete":
		first.Requirements.DepthComplete = true
	case "claim-surface-pattern-complete":
		first.Requirements.SurfacePatternComplete = true
	case "claim-atomic-authority":
		first.Requirements.AuthorityAtomicBinding = true
	case "aggregate-rejections":
		lock.DefinitiveEligible = true
		lock.CompletionState = "complete"
	case "drop-candidate":
		lock.Candidates = lock.Candidates[:len(lock.Candidates)-1]
	case "substitute-fixture":
		first.SubjectID = "fixture-http-source"
	case "public-completion-effect":
		lock.VerificationBoundary.CompletionEffect = "definitive"
	case "source-commit-drift":
		first.Commit = strings.Repeat("0", 40)
	case "certificate-digest-drift":
		first.Certificate.Digest = "sha256:" + strings.Repeat("0", 64)
	default:
		return fmt.Errorf("未知のSubject binding mutationです: %s", operation)
	}
	return nil
}
