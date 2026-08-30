package lab

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type PreviewComposition struct {
	SchemaVersion        int                 `json:"schema_version"`
	ID                   string              `json:"id"`
	Stage                int                 `json:"stage"`
	CoreContract         PreviewCoreContract `json:"core_contract"`
	CompletionMode       string              `json:"completion_mode"`
	CoverageEpoch        string              `json:"coverage_epoch"`
	DepthReferenceLock   PreviewArtifactLock `json:"depth_reference_lock"`
	ScenarioContractLock PreviewArtifactLock `json:"scenario_contract_lock"`
	SubjectDepthParity   PreviewArtifactLock `json:"subject_depth_parity"`
	IntegrationProofs    PreviewArtifactLock `json:"integration_proofs"`
	Subjects             []PreviewSubjectRef `json:"subjects"`
}

type PreviewCoreContract struct {
	Repository    string `json:"repository"`
	BaseCommit    string `json:"base_commit"`
	PolicyVersion string `json:"policy_version"`
	Status        string `json:"status"`
}

type PreviewSubjectRef struct {
	Name              string `json:"name"`
	SubjectID         string `json:"subject_id"`
	Version           string `json:"version"`
	Certificate       string `json:"certificate"`
	CertificateDigest string `json:"certificate_digest"`
}

type PreviewSubjectCertificate struct {
	SchemaVersion     int                `json:"schema_version"`
	SubjectID         string             `json:"subject_id"`
	Version           string             `json:"version"`
	Status            string             `json:"status"`
	CompletionClass   string             `json:"completion_class"`
	CorePolicyVersion string             `json:"core_policy_version"`
	CoverageEpoch     string             `json:"coverage_epoch"`
	CoverageStates    []string           `json:"coverage_states"`
	ArtifactDigest    string             `json:"artifact_digest"`
	ValidFrom         string             `json:"valid_from"`
	ValidUntil        string             `json:"valid_until"`
	SupersedesDigest  string             `json:"supersedes_digest,omitempty"`
	Revocation        *PreviewRevocation `json:"revocation,omitempty"`
}

type PreviewRevocation struct {
	RevokedAt string `json:"revoked_at"`
	Reason    string `json:"reason"`
}

type DefinitiveWarning struct {
	Code    string `json:"code"`
	Subject string `json:"subject,omitempty"`
	Message string `json:"message"`
}

type DefinitiveGateResult struct {
	SchemaVersion              int                 `json:"schema_version"`
	CompositionID              string              `json:"composition_id"`
	RequestedMode              string              `json:"requested_mode"`
	CoreV2Status               string              `json:"core_v2_status"`
	EffectiveState             string              `json:"effective_state"`
	DefinitiveEligible         bool                `json:"definitive_eligible"`
	LegacyBundlePreserved      bool                `json:"legacy_bundle_preserved"`
	DepthReferenceStatus       string              `json:"depth_reference_status"`
	DepthParityEligible        bool                `json:"depth_parity_eligible"`
	IntegrationProofsValid     bool                `json:"integration_proofs_valid"`
	IntegratedScenariosPassed  int                 `json:"integrated_scenarios_passed"`
	SurfacePatternRows         int                 `json:"surface_pattern_rows"`
	PatternSpecificRows        int                 `json:"pattern_specific_rows"`
	RuntimeIdentityRows        int                 `json:"runtime_identity_rows"`
	PatternSpecificCaptureRows int                 `json:"pattern_specific_capture_rows"`
	SurfacePatternGaps         int                 `json:"surface_pattern_gaps"`
	AuthorityAtomicRows        int                 `json:"authority_atomic_rows"`
	SurfacePatternEligible     int                 `json:"surface_pattern_eligible"`
	Warnings                   []DefinitiveWarning `json:"warnings"`
}

type CertificateOverride struct {
	Version           string `json:"version"`
	Certificate       string `json:"certificate"`
	CertificateDigest string `json:"certificate_digest"`
}

type DefinitiveMatrix struct {
	SchemaVersion int                    `json:"schema_version"`
	CoreV2Status  string                 `json:"core_v2_status"`
	AsOf          string                 `json:"as_of"`
	Cases         []DefinitiveMatrixCase `json:"cases"`
}

type DefinitiveMatrixCase struct {
	ID                         string                         `json:"id"`
	Composition                string                         `json:"composition"`
	CertificateOverrides       map[string]CertificateOverride `json:"certificate_overrides,omitempty"`
	ExpectedState              string                         `json:"expected_state"`
	ExpectedDefinitiveEligible bool                           `json:"expected_definitive_eligible"`
	ExpectedWarnings           []string                       `json:"expected_warnings"`
}

type DefinitiveMatrixResult struct {
	SchemaVersion int                    `json:"schema_version"`
	CoreV2Status  string                 `json:"core_v2_status"`
	Cases         []DefinitiveCaseResult `json:"cases"`
	Verdict       string                 `json:"verdict"`
}

type DefinitiveCaseResult struct {
	ID             string   `json:"id"`
	EffectiveState string   `json:"effective_state"`
	Warnings       []string `json:"warnings"`
	Verdict        string   `json:"verdict"`
}

func EvaluateDefinitiveComposition(root, compositionPath, asOf string) (DefinitiveGateResult, error) {
	if _, err := NonRegressionGate(root); err != nil {
		return DefinitiveGateResult{}, err
	}
	return evaluateDefinitiveComposition(root, compositionPath, asOf, nil)
}

func evaluateDefinitiveComposition(root, compositionPath, asOf string, overrides map[string]CertificateOverride) (DefinitiveGateResult, error) {
	return evaluateDefinitiveCompositionWithPinnedRoot(root, root, compositionPath, asOf, overrides)
}

func evaluateDefinitiveCompositionWithPinnedRoot(root, pinnedRepositoryRoot, compositionPath, asOf string, overrides map[string]CertificateOverride) (DefinitiveGateResult, error) {
	var composition PreviewComposition
	if err := LoadJSON(resolve(root, compositionPath), &composition); err != nil {
		return DefinitiveGateResult{}, err
	}
	if composition.SchemaVersion != 2 || composition.Stage != 2 || len(composition.Subjects) < 2 {
		return DefinitiveGateResult{}, fmt.Errorf("v2 Preview Compositionはschema_version=2、stage=2、2 Subject以上が必須です")
	}
	if composition.CompletionMode != "bounded-complete" && composition.CompletionMode != "subject-definitive" {
		return DefinitiveGateResult{}, fmt.Errorf("未知のCompletion Modeです: %s", composition.CompletionMode)
	}
	if composition.CoreContract.Status != "draft" || composition.CoreContract.PolicyVersion != "2.0.0-draft.1" {
		return DefinitiveGateResult{}, fmt.Errorf("Core v2未確定中はdraft Preview契約だけを受理します")
	}
	if composition.CoreContract.BaseCommit != evidenceDependencyCoreCommit {
		return DefinitiveGateResult{}, fmt.Errorf("Core Evidence Dependency確定main Commitと一致しません")
	}
	if err := verifyPinnedCoreCommit(pinnedRepositoryRoot, composition.CoreContract.Repository, composition.CoreContract.BaseCommit); err != nil {
		return DefinitiveGateResult{}, err
	}
	instant, err := time.Parse(time.RFC3339, asOf)
	if err != nil {
		return DefinitiveGateResult{}, fmt.Errorf("as-ofがRFC3339ではありません: %w", err)
	}
	result := DefinitiveGateResult{
		SchemaVersion:         1,
		CompositionID:         composition.ID,
		RequestedMode:         composition.CompletionMode,
		CoreV2Status:          composition.CoreContract.Status,
		DefinitiveEligible:    composition.CompletionMode == "subject-definitive",
		LegacyBundlePreserved: true,
		Warnings:              []DefinitiveWarning{},
	}
	depthComplete, err := evaluateCompositionDepth(root, pinnedRepositoryRoot, composition, &result)
	if err != nil {
		return DefinitiveGateResult{}, err
	}
	if !depthComplete {
		result.DefinitiveEligible = false
	}
	epochComplete := depthComplete
	schemaVersions := map[int]bool{}
	releaseVersions := map[string]bool{}
	seenSubjects := map[string]bool{}
	if composition.CompletionMode == "bounded-complete" {
		result.DefinitiveEligible = false
		addDefinitiveWarning(&result, "bounded-scope-only", "", "固定Epoch内の完了でありSubject Definitiveではありません。")
	}
	for _, originalRef := range composition.Subjects {
		ref := originalRef
		if override, ok := overrides[ref.Name]; ok {
			ref.Version = override.Version
			ref.Certificate = override.Certificate
			ref.CertificateDigest = override.CertificateDigest
		}
		if seenSubjects[ref.Name] {
			return DefinitiveGateResult{}, fmt.Errorf("Subject名が重複しています: %s", ref.Name)
		}
		seenSubjects[ref.Name] = true
		releaseVersions[ref.Version] = true
		certificatePath := resolve(root, ref.Certificate)
		actualDigest, err := DigestFile(certificatePath)
		if err != nil {
			return DefinitiveGateResult{}, err
		}
		if actualDigest != ref.CertificateDigest {
			return DefinitiveGateResult{}, fmt.Errorf("Subject %sのCertificate Digest不一致", ref.Name)
		}
		schemaVersion, err := certificateSchemaVersion(certificatePath)
		if err != nil {
			return DefinitiveGateResult{}, err
		}
		schemaVersions[schemaVersion] = true
		switch schemaVersion {
		case 1:
			var certificate SubjectCertificate
			if err := LoadJSON(certificatePath, &certificate); err != nil {
				return DefinitiveGateResult{}, err
			}
			if certificate.SubjectID != ref.SubjectID || certificate.Version != ref.Version || certificate.Status != "complete" {
				epochComplete = false
				result.DefinitiveEligible = false
				addDefinitiveWarning(&result, "legacy-certificate-invalid", ref.Name, "v1 Certificateが固定Subject Releaseを証明していません。")
			} else {
				result.DefinitiveEligible = false
				addDefinitiveWarning(&result, "legacy-v1-certificate", ref.Name, "v1 Certificateは旧Bundle検証にだけ使用でき、Definitiveへ昇格できません。")
			}
		case 2:
			var certificate PreviewSubjectCertificate
			if err := LoadJSON(certificatePath, &certificate); err != nil {
				return DefinitiveGateResult{}, err
			}
			if certificate.SubjectID != ref.SubjectID || certificate.Version != ref.Version || certificate.CorePolicyVersion != composition.CoreContract.PolicyVersion || certificate.CoverageEpoch != composition.CoverageEpoch {
				return DefinitiveGateResult{}, fmt.Errorf("Subject %sのv2 Certificate LockがCompositionと一致しません", ref.Name)
			}
			if err := evaluateV2Certificate(root, ref.Name, certificate, instant, &result, &epochComplete); err != nil {
				return DefinitiveGateResult{}, err
			}
		default:
			return DefinitiveGateResult{}, fmt.Errorf("未対応Certificate schema_version=%d", schemaVersion)
		}
	}
	if len(schemaVersions) > 1 {
		result.DefinitiveEligible = false
		addDefinitiveWarning(&result, "mixed-certificate-versions", "", "v1とv2 Certificateが混在しています。")
	}
	if len(releaseVersions) > 1 {
		addDefinitiveWarning(&result, "mixed-subject-release-versions", "", "Subject Release Versionが混在しているため固定組合せとして明示します。")
	}
	if !epochComplete {
		result.DefinitiveEligible = false
		result.EffectiveState = "incomplete"
	} else if result.DefinitiveEligible {
		result.EffectiveState = "definitive-candidate"
	} else {
		result.EffectiveState = "bounded-complete"
	}
	if composition.CompletionMode == "subject-definitive" && !result.DefinitiveEligible {
		addDefinitiveWarning(&result, "state-downgraded", "", "Definitive要件を満たさないため状態を昇格せず降格しました。")
	}
	addDefinitiveWarning(&result, "core-v2-draft", "", "Core v2未確定のためDefinitive completeは発行できません。")
	return result, nil
}

func evaluateV2Certificate(root, subject string, certificate PreviewSubjectCertificate, asOf time.Time, result *DefinitiveGateResult, epochComplete *bool) error {
	if certificate.CompletionClass != "bounded-complete" && certificate.CompletionClass != "subject-definitive" {
		return fmt.Errorf("Subject %sのCompletion Classが不正です", subject)
	}
	if len(certificate.CoverageStates) == 0 || !sha256Pattern.MatchString(certificate.ArtifactDigest) {
		return fmt.Errorf("Subject %sのCoverageまたはArtifact Digestが不正です", subject)
	}
	from, err := time.Parse(time.RFC3339, certificate.ValidFrom)
	if err != nil {
		return fmt.Errorf("Subject %sのvalid_fromが不正です", subject)
	}
	until, err := time.Parse(time.RFC3339, certificate.ValidUntil)
	if err != nil || !until.After(from) {
		return fmt.Errorf("Subject %sのvalid_untilが不正です", subject)
	}
	if certificate.Status == "revoked" {
		if certificate.Revocation == nil || certificate.Revocation.Reason == "" {
			return fmt.Errorf("Subject %sの失効情報が不足しています", subject)
		}
		revokedAt, revokedErr := time.Parse(time.RFC3339, certificate.Revocation.RevokedAt)
		if revokedErr != nil || revokedAt.Before(from) || revokedAt.After(asOf) {
			return fmt.Errorf("Subject %sのrevoked_atが不正です", subject)
		}
		*epochComplete = false
		result.DefinitiveEligible = false
		addDefinitiveWarning(result, "certificate-revoked", subject, "失効済みCertificateはCompositionへ使用できません。")
	} else if certificate.Status != "active" {
		*epochComplete = false
		result.DefinitiveEligible = false
		addDefinitiveWarning(result, "certificate-inactive", subject, "ActiveでないCertificateはCompositionへ使用できません。")
	}
	if asOf.Before(from) || !asOf.Before(until) {
		*epochComplete = false
		result.DefinitiveEligible = false
		addDefinitiveWarning(result, "certificate-expired", subject, "評価時点でCertificateが有効期間外です。")
	}
	if certificate.CompletionClass != "subject-definitive" {
		result.DefinitiveEligible = false
		addDefinitiveWarning(result, "not-subject-definitive", subject, "Certificateはbounded-complete（固定Epoch完了）です。")
	}
	for _, state := range certificate.CoverageStates {
		switch state {
		case "covered":
		case "excluded":
			result.DefinitiveEligible = false
			addDefinitiveWarning(result, "coverage-excluded", subject, "excluded Coverageを含むためDefinitiveへ昇格できません。")
		case "infeasible":
			result.DefinitiveEligible = false
			addDefinitiveWarning(result, "coverage-infeasible", subject, "infeasible Coverageを含むためDefinitiveへ昇格できません。")
		case "partial":
			*epochComplete = false
			result.DefinitiveEligible = false
			addDefinitiveWarning(result, "coverage-partial", subject, "partial Coverageを含むためEpoch完了にもできません。")
		default:
			return fmt.Errorf("Subject %sに未知のCoverage Stateがあります: %s", subject, state)
		}
	}
	if certificate.SupersedesDigest != "" {
		previous, err := findSupersededCertificate(root, certificate.SubjectID, certificate.SupersedesDigest)
		if err != nil {
			return err
		}
		if previous.Status != "revoked" {
			return fmt.Errorf("更新元Certificateがrevokedではありません: %s", certificate.SubjectID)
		}
		addDefinitiveWarning(result, "certificate-renewed", subject, "失効CertificateをDigest指定で更新しています。")
	}
	return nil
}

func certificateSchemaVersion(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var header struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return 0, err
	}
	return header.SchemaVersion, nil
}

var sha256Pattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func findSupersededCertificate(root, subjectID, digest string) (PreviewSubjectCertificate, error) {
	var found PreviewSubjectCertificate
	err := filepath.Walk(filepath.Join(root, "fixtures", "definitive-v2"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".json" {
			return err
		}
		actual, digestErr := DigestFile(path)
		if digestErr != nil || actual != digest {
			return digestErr
		}
		if loadErr := LoadJSON(path, &found); loadErr != nil {
			return loadErr
		}
		if found.SubjectID != subjectID {
			return fmt.Errorf("更新元CertificateのSubject IDが一致しません")
		}
		return filepath.SkipAll
	})
	if err != nil {
		return PreviewSubjectCertificate{}, err
	}
	if found.SubjectID == "" {
		return PreviewSubjectCertificate{}, fmt.Errorf("更新元Certificate Digestが見つかりません: %s", digest)
	}
	return found, nil
}

func addDefinitiveWarning(result *DefinitiveGateResult, code, subject, message string) {
	for _, warning := range result.Warnings {
		if warning.Code == code && warning.Subject == subject {
			return
		}
	}
	result.Warnings = append(result.Warnings, DefinitiveWarning{Code: code, Subject: subject, Message: message})
}

func RunDefinitiveMatrix(root, matrixPath string) (DefinitiveMatrixResult, error) {
	if _, err := NonRegressionGate(root); err != nil {
		return DefinitiveMatrixResult{}, err
	}
	var matrix DefinitiveMatrix
	if err := LoadJSON(resolve(root, matrixPath), &matrix); err != nil {
		return DefinitiveMatrixResult{}, err
	}
	if matrix.SchemaVersion != 1 || matrix.CoreV2Status != "draft" || len(matrix.Cases) == 0 {
		return DefinitiveMatrixResult{}, fmt.Errorf("Definitive Gate v2 Matrix契約が不正です")
	}
	report := DefinitiveMatrixResult{SchemaVersion: 1, CoreV2Status: matrix.CoreV2Status, Cases: []DefinitiveCaseResult{}, Verdict: "pass"}
	for _, testCase := range matrix.Cases {
		result, err := evaluateDefinitiveComposition(root, testCase.Composition, matrix.AsOf, testCase.CertificateOverrides)
		if err != nil {
			return report, fmt.Errorf("Fixture %s: %w", testCase.ID, err)
		}
		codes := make([]string, 0, len(result.Warnings))
		for _, warning := range result.Warnings {
			if !contains(codes, warning.Code) {
				codes = append(codes, warning.Code)
			}
		}
		sort.Strings(codes)
		expected := append([]string(nil), testCase.ExpectedWarnings...)
		sort.Strings(expected)
		verdict := "pass"
		if result.EffectiveState != testCase.ExpectedState || result.DefinitiveEligible != testCase.ExpectedDefinitiveEligible || !sameSet(codes, expected) || result.EffectiveState == "definitive-complete" {
			verdict = "fail"
			report.Verdict = "fail"
		}
		report.Cases = append(report.Cases, DefinitiveCaseResult{ID: testCase.ID, EffectiveState: result.EffectiveState, Warnings: codes, Verdict: verdict})
	}
	if report.Verdict != "pass" {
		return report, fmt.Errorf("Definitive Gate v2 Fixture Matrixがfailです")
	}
	return report, nil
}

type V2MigrationReport struct {
	SchemaVersion                int                  `json:"schema_version"`
	SourceCompositionID          string               `json:"source_composition_id"`
	SourceDigest                 string               `json:"source_digest"`
	TargetPolicy                 string               `json:"target_policy"`
	CoreV2Status                 string               `json:"core_v2_status"`
	MigrationState               string               `json:"migration_state"`
	WritesPerformed              bool                 `json:"writes_performed"`
	DepthReferenceCommit         string               `json:"depth_reference_commit"`
	CoreEvidenceDependencyCommit string               `json:"core_evidence_dependency_commit"`
	RequiredDepthAxes            int                  `json:"required_depth_axes"`
	RequiredSurfacePatternRows   int                  `json:"required_surface_pattern_rows"`
	OpenSurfacePatternGaps       int                  `json:"open_surface_pattern_gaps"`
	Subjects                     []V2MigrationSubject `json:"subjects"`
	Warnings                     []string             `json:"warnings"`
}

type V2MigrationSubject struct {
	Name          string `json:"name"`
	SubjectID     string `json:"subject_id"`
	CurrentSchema int    `json:"current_certificate_schema"`
	Action        string `json:"action"`
}

func PlanV2Migration(root, compositionPath string) (V2MigrationReport, error) {
	if _, err := NonRegressionGate(root); err != nil {
		return V2MigrationReport{}, err
	}
	if err := ValidateDepthInheritance(root); err != nil {
		return V2MigrationReport{}, err
	}
	validated, err := Preflight(root, compositionPath, "")
	if err != nil {
		return V2MigrationReport{}, err
	}
	report := V2MigrationReport{
		SchemaVersion:                1,
		SourceCompositionID:          validated.Manifest.ID,
		SourceDigest:                 validated.Digest,
		TargetPolicy:                 "2.0.0-draft.1",
		CoreV2Status:                 "draft",
		MigrationState:               "requires-depth-parity-and-certificate-renewal",
		WritesPerformed:              false,
		DepthReferenceCommit:         feDepthReferenceCommit,
		CoreEvidenceDependencyCommit: evidenceDependencyCoreCommit,
		RequiredDepthAxes:            18,
		RequiredSurfacePatternRows:   850,
		OpenSurfacePatternGaps:       421,
		Subjects:                     []V2MigrationSubject{},
		Warnings:                     []string{"legacy-bundle-remains-verifiable", "core-v2-draft", "no-definitive-promotion", "subject-depth-parity-required", "integration-proof-not-substitute", "surface-pattern-proof-closure-required", "evidence-dependency-consumer-matrix-required"},
	}
	for _, subject := range validated.Manifest.Subjects {
		report.Subjects = append(report.Subjects, V2MigrationSubject{Name: subject.Name, SubjectID: subject.SubjectID, CurrentSchema: 1, Action: "complete-18-axis-depth-parity-close-surface-pattern-proofs-verify-evidence-dependency-and-issue-v2-subject-certificate"})
	}
	return report, nil
}

type DefinitivePreviewAudit struct {
	SchemaVersion int              `json:"schema_version"`
	Audit         string           `json:"audit"`
	Checks        []SelfAuditCheck `json:"checks"`
	Verdict       string           `json:"verdict"`
}

func AuditDefinitivePreview(root string) DefinitivePreviewAudit {
	report := DefinitivePreviewAudit{SchemaVersion: 1, Audit: "definitive-gate-v2-preview", Checks: []SelfAuditCheck{}, Verdict: "pass"}
	branchOutput, branchErr := exec.Command("git", "-C", root, "branch", "--show-current").Output()
	if branchErr == nil && strings.TrimSpace(string(branchOutput)) == "main" {
		branchErr = fmt.Errorf("Core v2確定前のPreviewをmainで完成扱いにできません")
	}
	report.add("feature-branch", branchErr)
	_, matrixErr := RunDefinitiveMatrix(root, "tests/fixtures/definitive-gate-v2.matrix.json")
	report.add("fixture-matrix", matrixErr)
	migration, migrationErr := PlanV2Migration(root, "compositions/fixture-stage2.json")
	if migrationErr == nil && (migration.WritesPerformed || migration.CoreV2Status != "draft") {
		migrationErr = fmt.Errorf("Migration Previewが非破壊draftではありません")
	}
	report.add("non-destructive-migration", migrationErr)
	report.add("legacy-v1-bundle", ValidateLegacyV1Bundle(root))
	var routerEval map[string]any
	routerErr := LoadJSON(filepath.Join(root, "evals", "preview", "interoperability-router.definitive-v2-preview.json"), &routerEval)
	if routerErr == nil && (routerEval["verdict"] != "pass" || routerEval["core_v2_status"] != "draft") {
		routerErr = fmt.Errorf("Definitive v2 Router Evalがpass/draftではありません")
	}
	report.add("router-eval", routerErr)
	var composition PreviewComposition
	lockErr := LoadJSON(filepath.Join(root, "compositions", "fixture-stage2-v2-definitive.preview.json"), &composition)
	if lockErr == nil && (composition.CoreContract.Status != "draft" || composition.CoreContract.PolicyVersion != "2.0.0-draft.1" || composition.CoreContract.BaseCommit != evidenceDependencyCoreCommit) {
		lockErr = fmt.Errorf("Core v2 Preview Lockがdraftではありません")
	}
	report.add("core-v2-draft-lock", lockErr)
	consumerMatrix, consumerMatrixErr := RunEvidenceDependencyConsumerMatrix(root, "tests/fixtures/evidence-dependency-consumer.matrix.json")
	if consumerMatrixErr == nil && (consumerMatrix.Verdict != "pass" || consumerMatrix.CoreCommit != evidenceDependencyCoreCommit || consumerMatrix.CoreStatus != "main-ci-confirmed" || len(consumerMatrix.Results) != 21) {
		consumerMatrixErr = fmt.Errorf("Evidence Dependency consumer互換性Matrixが確定main契約を保持していません")
	}
	report.add("evidence-dependency-consumer-matrix", consumerMatrixErr)
	report.add("runtime-binding-local", ValidateSavedRuntimeBindingEvidence(root, "local"))
	report.add("runtime-binding-container", ValidateSavedRuntimeBindingEvidence(root, "container"))
	report.add("composition-evidence-dependency-closure", ValidateCompositionEvidenceClosure(root))
	subjectBinding, subjectBindingErr := EvaluateSubjectBindingAdmission(root, root)
	if subjectBindingErr == nil && (subjectBinding.Verdict != "pass" || subjectBinding.CompletionState != "incomplete" || subjectBinding.DefinitiveEligible || len(subjectBinding.Candidates) != 3 || subjectBinding.NegativeCases != 14) {
		subjectBindingErr = fmt.Errorf("Actual Subject binding admissionが未完成候補を保持していません")
	}
	report.add("actual-subject-binding-admission", subjectBindingErr)
	var compositionMatrix CompositionCompatibilityResult
	compositionMatrixErr := LoadJSON(filepath.Join(root, "evidence", "preview", "composition-compatibility.matrix.json"), &compositionMatrix)
	if compositionMatrixErr == nil {
		if compositionMatrix.Verdict != "pass" || compositionMatrix.CoreCommit != evidenceDependencyCoreCommit || len(compositionMatrix.Results) != 27 {
			compositionMatrixErr = fmt.Errorf("複数Subject Composition互換性Evidenceが不正です")
		}
		multipleSubjectFailureResults := 0
		for _, result := range compositionMatrix.Results {
			if result.DefinitiveEligible || !result.ConsumerIndependent || !containsAll(result.InheritedGaps, []string{"core-v2-draft", "subject-depth-parity-incomplete", "subject-probe-atomic-binding-gap", "surface-pattern-proof-gaps"}) {
				compositionMatrixErr = fmt.Errorf("複数Subject CompositionがGapを隠しています")
				break
			}
			if result.CaseID == "reject-multiple-subject-failures-without-aggregation" {
				if result.CompatibilityState != "reject" || result.ClaimGraphState != "pass" || !sameSet(result.FailedSubjects, []string{"source", "sink"}) {
					compositionMatrixErr = fmt.Errorf("複数Subject同時失敗が個別結果を保持していません")
					break
				}
				for _, subject := range result.Subjects {
					if subject.GateState != "reject" || subject.CertificateState != "reject" {
						compositionMatrixErr = fmt.Errorf("複数Subject同時失敗がSubject probe拒否を保持していません")
						break
					}
				}
				multipleSubjectFailureResults++
			}
		}
		if compositionMatrixErr == nil && multipleSubjectFailureResults != 3 {
			compositionMatrixErr = fmt.Errorf("複数Subject同時失敗Evidenceのconsumer網羅が不足しています")
		}
	}
	report.add("multi-subject-composition-compatibility", compositionMatrixErr)
	_, nonRegressionErr := NonRegressionGate(root)
	report.add("interop-non-regression", nonRegressionErr)
	report.add("neutral-language", ValidateNeutralLanguage(root))
	report.add("subject-depth-parity-inheritance", ValidateDepthInheritance(root))
	var depthResult DefinitiveGateResult
	depthEvidenceErr := LoadJSON(filepath.Join(root, "evidence", "preview", "depth-parity.result.json"), &depthResult)
	if depthEvidenceErr == nil && (depthResult.EffectiveState != "incomplete" || depthResult.DefinitiveEligible || depthResult.DepthParityEligible || !depthResult.IntegrationProofsValid || depthResult.DepthReferenceStatus != "incomplete" || depthResult.IntegratedScenariosPassed != 10 || depthResult.SurfacePatternRows != 850 || depthResult.PatternSpecificRows != 429 || depthResult.RuntimeIdentityRows != 170 || depthResult.PatternSpecificCaptureRows != 259 || depthResult.SurfacePatternGaps != 421 || depthResult.SurfacePatternEligible != 0 || depthResult.AuthorityAtomicRows != 0) {
		depthEvidenceErr = fmt.Errorf("Depth Parity Preview Evidenceが不足継承を記録していません")
	}
	report.add("depth-parity-evidence", depthEvidenceErr)
	return report
}

func (report *DefinitivePreviewAudit) add(name string, err error) {
	check := SelfAuditCheck{Name: name, Verdict: "pass", Detail: "検証済み"}
	if err != nil {
		check.Verdict = "fail"
		check.Detail = err.Error()
		report.Verdict = "fail"
	}
	report.Checks = append(report.Checks, check)
}

func verifyPinnedCoreCommit(root, repository, commit string) error {
	core := filepath.Join(root, "..", repository)
	cmd := exec.Command("git", "-C", core, "cat-file", "-e", commit+"^{commit}")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("固定Core Commitを検証できません: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
