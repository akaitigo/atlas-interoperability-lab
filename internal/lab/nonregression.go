package lab

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const nonRegressionBaselineCommit = "1894452124e722955576a6a3744da7744c68f6f9"
const nonRegressionBaselineDigest = "sha256:ee2b3b91fcfde12d489022133b964481a0611b08b3613466cf39115527e0939b"

type NonRegressionBaseline struct {
	SchemaVersion       int      `json:"schema_version"`
	ID                  string   `json:"id"`
	SourceCommit        string   `json:"source_commit"`
	CompositionPath     string   `json:"composition_path"`
	RequiredProfiles    []string `json:"required_profiles"`
	ImmutablePrefixes   []string `json:"immutable_prefixes"`
	ImmutablePaths      []string `json:"immutable_paths"`
	CIPath              string   `json:"ci_path"`
	ReplacementManifest string   `json:"replacement_manifest"`
}

type NonRegressionMigration struct {
	SchemaVersion int                    `json:"schema_version"`
	BaselineID    string                 `json:"baseline_id"`
	Mappings      []NonRegressionMapping `json:"mappings"`
}

type NonRegressionMapping struct {
	OldID             string        `json:"old_id"`
	OldPath           string        `json:"old_path"`
	ReplacementIDs    []string      `json:"replacement_ids"`
	ReplacementPaths  []string      `json:"replacement_paths"`
	IntegrationProofs []EvidenceRef `json:"integration_proofs"`
	MigrationEvidence []EvidenceRef `json:"migration_evidence"`
}

type EvidenceRef struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Digest  string `json:"digest"`
	Profile string `json:"profile"`
}

type NonRegressionGateReport struct {
	SchemaVersion  int              `json:"schema_version"`
	Gate           string           `json:"gate"`
	BaselineCommit string           `json:"baseline_commit"`
	Checks         []SelfAuditCheck `json:"checks"`
	Verdict        string           `json:"verdict"`
}

type NonRegressionViolation struct {
	Code   string
	Detail string
}

func (violation *NonRegressionViolation) Error() string {
	return violation.Code + ": " + violation.Detail
}

type repositoryReader func(path string) ([]byte, error)

func NonRegressionGate(root string) (NonRegressionGateReport, error) {
	return nonRegressionGate(root, func(path string) ([]byte, error) {
		return os.ReadFile(resolve(root, path))
	})
}

func nonRegressionGate(root string, reader repositoryReader) (NonRegressionGateReport, error) {
	report := NonRegressionGateReport{SchemaVersion: 1, Gate: "interop-non-regression", BaselineCommit: nonRegressionBaselineCommit, Checks: []SelfAuditCheck{}, Verdict: "pass"}
	baseline, migrations, err := loadNonRegressionPolicy(root, reader)
	if err != nil {
		return failNonRegression(report, err)
	}
	report.Checks = append(report.Checks, SelfAuditCheck{Name: "sealed-baseline", Verdict: "pass", Detail: "main固定CommitとManifest Digestを検証"})
	if err := validateRepositoryContract(reader); err != nil {
		return failNonRegression(report, err)
	}
	report.Checks = append(report.Checks, SelfAuditCheck{Name: "repository-contract", Verdict: "pass", Detail: "正規fleet contractと書込み境界を検証"})
	if err := validateNeutralLanguage(root, reader); err != nil {
		return failNonRegression(report, err)
	}
	report.Checks = append(report.Checks, SelfAuditCheck{Name: "neutral-language", Verdict: "pass", Detail: "Docs・Scenario・Metadataの中立表現と技術的namespace参照を検証"})
	baselineCompositionData, err := gitFile(root, baseline.SourceCommit, baseline.CompositionPath)
	if err != nil {
		return failNonRegression(report, err)
	}
	var baselineComposition Composition
	if err := decodeStrictJSON(baseline.CompositionPath, baselineCompositionData, &baselineComposition); err != nil {
		return failNonRegression(report, err)
	}
	if err := scanForbiddenControls(reader, append([]string{baseline.CompositionPath, baseline.CIPath}, append(baseline.ImmutablePaths, baselineComposition.Scenarios...)...)); err != nil {
		return failNonRegression(report, err)
	}
	report.Checks = append(report.Checks, SelfAuditCheck{Name: "no-skip-disable-optional", Verdict: "pass", Detail: "禁止Controlなし"})
	currentCompositionData, err := reader(baseline.CompositionPath)
	if err != nil {
		return failNonRegression(report, violation("composition-removed", "Compositionがありません"))
	}
	var currentComposition Composition
	if err := decodeStrictJSON(baseline.CompositionPath, currentCompositionData, &currentComposition); err != nil {
		return failNonRegression(report, violation("composition-regressed", err.Error()))
	}
	if err := compareComposition(root, reader, baselineComposition, currentComposition, migrations); err != nil {
		return failNonRegression(report, err)
	}
	report.Checks = append(report.Checks, SelfAuditCheck{Name: "composition-components-versions", Verdict: "pass", Detail: fmt.Sprintf("subjects=%d profiles=%d", len(baselineComposition.Subjects), len(baseline.RequiredProfiles))})
	for _, scenarioPath := range baselineComposition.Scenarios {
		baselineData, err := gitFile(root, baseline.SourceCommit, scenarioPath)
		if err != nil {
			return failNonRegression(report, err)
		}
		currentData, err := reader(scenarioPath)
		if err != nil {
			replacementErr := validateReplacement(root, reader, migrations, "scenario:"+strings.TrimSuffix(filepath.Base(scenarioPath), filepath.Ext(scenarioPath)), scenarioPath)
			if replacementErr == nil {
				continue
			}
			if !isMissingReplacement(replacementErr) {
				return failNonRegression(report, replacementErr)
			}
			return failNonRegression(report, violation("scenario-removed", scenarioPath))
		}
		var before, after Scenario
		if err := decodeStrictJSON(scenarioPath, baselineData, &before); err != nil {
			return failNonRegression(report, err)
		}
		if err := decodeStrictJSON(scenarioPath, currentData, &after); err != nil {
			return failNonRegression(report, violation("scenario-regressed", err.Error()))
		}
		if err := compareScenario(before, after); err != nil {
			replacementErr := validateReplacement(root, reader, migrations, "scenario:"+before.ID, scenarioPath)
			if replacementErr == nil {
				continue
			}
			if !isMissingReplacement(replacementErr) {
				return failNonRegression(report, replacementErr)
			}
			return failNonRegression(report, err)
		}
	}
	report.Checks = append(report.Checks, SelfAuditCheck{Name: "scenario-action-assertion-threshold", Verdict: "pass", Detail: fmt.Sprintf("scenarios=%d", len(baselineComposition.Scenarios))})
	exactPaths, err := baselineExactPaths(root, baseline)
	if err != nil {
		return failNonRegression(report, err)
	}
	for _, path := range exactPaths {
		before, err := gitFile(root, baseline.SourceCommit, path)
		if err != nil {
			return failNonRegression(report, err)
		}
		after, err := reader(path)
		if err != nil {
			replacementErr := validateReplacement(root, reader, migrations, "artifact:"+path, path)
			if replacementErr == nil {
				continue
			}
			if !isMissingReplacement(replacementErr) {
				return failNonRegression(report, replacementErr)
			}
			return failNonRegression(report, violation("artifact-removed", path))
		}
		if !bytes.Equal(before, after) {
			replacementErr := validateReplacement(root, reader, migrations, "artifact:"+path, path)
			if replacementErr == nil {
				continue
			}
			if !isMissingReplacement(replacementErr) {
				return failNonRegression(report, replacementErr)
			}
			return failNonRegression(report, violation("artifact-regressed", path))
		}
	}
	report.Checks = append(report.Checks, SelfAuditCheck{Name: "contracts-tests-integration-evidence", Verdict: "pass", Detail: fmt.Sprintf("immutable_artifacts=%d", len(exactPaths))})
	baselineCI, err := gitFile(root, baseline.SourceCommit, baseline.CIPath)
	if err != nil {
		return failNonRegression(report, err)
	}
	currentCI, err := reader(baseline.CIPath)
	if err != nil || !lineMultisetContains(currentCI, baselineCI) || !ciStepSuperset(currentCI, baselineCI) {
		return failNonRegression(report, violation("ci-regressed", "既存CI StepまたはCommandが削減されました"))
	}
	report.Checks = append(report.Checks, SelfAuditCheck{Name: "ci-matrix-superset", Verdict: "pass", Detail: "baseline CI行を包含"})
	return report, nil
}

func loadNonRegressionPolicy(root string, reader repositoryReader) (NonRegressionBaseline, NonRegressionMigration, error) {
	var baseline NonRegressionBaseline
	data, err := reader("baselines/interop-v1.non-regression.json")
	if err != nil {
		return baseline, NonRegressionMigration{}, err
	}
	if DigestBytes(data) != nonRegressionBaselineDigest {
		return baseline, NonRegressionMigration{}, violation("baseline-tampered", "Baseline Manifest Digestが固定値と一致しません")
	}
	if err := decodeStrictJSON("baselines/interop-v1.non-regression.json", data, &baseline); err != nil {
		return baseline, NonRegressionMigration{}, err
	}
	if baseline.SchemaVersion != 1 || baseline.ID != "interop-v1-non-regression" || baseline.SourceCommit != nonRegressionBaselineCommit || !sameSet(baseline.RequiredProfiles, []string{"local", "container"}) {
		return baseline, NonRegressionMigration{}, violation("baseline-tampered", "Baseline IdentityまたはProfileが不正です")
	}
	if output, gitErr := exec.Command("git", "-C", root, "cat-file", "-e", baseline.SourceCommit+"^{commit}").CombinedOutput(); gitErr != nil {
		return baseline, NonRegressionMigration{}, fmt.Errorf("Baseline Commitがありません: %w: %s", gitErr, strings.TrimSpace(string(output)))
	}
	var migrations NonRegressionMigration
	migrationData, err := reader(baseline.ReplacementManifest)
	if err != nil {
		return baseline, migrations, err
	}
	if err := decodeStrictJSON(baseline.ReplacementManifest, migrationData, &migrations); err != nil {
		return baseline, migrations, err
	}
	if migrations.SchemaVersion != 1 || migrations.BaselineID != baseline.ID {
		return baseline, migrations, violation("mapping-invalid", "Replacement MappingのBaseline IDが一致しません")
	}
	return baseline, migrations, nil
}

func compareComposition(root string, reader repositoryReader, before, after Composition, migrations NonRegressionMigration) error {
	if before.ID != after.ID || before.Stage != after.Stage || !isSuperset(after.Axes, before.Axes) || !isSuperset(after.Profiles, before.Profiles) {
		return violation("composition-regressed", "ID、Stage、AxesまたはProfileが縮小しました")
	}
	for _, scenarioPath := range before.Scenarios {
		if contains(after.Scenarios, scenarioPath) {
			continue
		}
		oldID := "scenario:" + strings.TrimSuffix(filepath.Base(scenarioPath), filepath.Ext(scenarioPath))
		if err := validateReplacement(root, reader, migrations, oldID, scenarioPath); err != nil {
			if !isMissingReplacement(err) {
				return err
			}
			return violation("scenario-removed", "Compositionから既存Scenarioが外されました: "+scenarioPath)
		}
		mapping := findReplacement(migrations, oldID, scenarioPath)
		if mapping == nil || !intersects(after.Scenarios, mapping.ReplacementPaths) {
			return violation("replacement-proof-insufficient", "Replacement ScenarioがCompositionに参照されていません: "+oldID)
		}
	}
	currentSubjects := map[string]SubjectRef{}
	for _, subject := range after.Subjects {
		currentSubjects[subject.Name] = subject
	}
	for _, old := range before.Subjects {
		current, ok := currentSubjects[old.Name]
		if !ok {
			replacementErr := validateReplacement(root, reader, migrations, "component:"+old.Name, old.ReleaseManifest)
			if replacementErr == nil {
				continue
			}
			if !isMissingReplacement(replacementErr) {
				return replacementErr
			}
			return violation("component-removed", old.Name)
		}
		if current.SubjectID != old.SubjectID || current.Version != old.Version || current.ReleaseManifest != old.ReleaseManifest || current.ReleaseDigest != old.ReleaseDigest || current.CertificateDigest != old.CertificateDigest {
			replacementErr := validateReplacement(root, reader, migrations, "component:"+old.Name, old.ReleaseManifest)
			if replacementErr == nil {
				continue
			}
			if !isMissingReplacement(replacementErr) {
				return replacementErr
			}
			return violation("component-regressed", old.Name+"のIdentity、VersionまたはDigestが変更されました")
		}
	}
	return nil
}

func compareScenario(before, after Scenario) error {
	if before.ID != after.ID || !isSuperset(after.Axes, before.Axes) || before.Oracle != after.Oracle {
		return violation("scenario-regressed", before.ID+"のID、AxisまたはOracleが変更されました")
	}
	beforePhases := map[string][]Action{"setup": before.Phases.Setup, "execute": before.Phases.Execute, "verify": before.Phases.Verify, "cleanup": before.Phases.Cleanup}
	afterPhases := map[string][]Action{"setup": after.Phases.Setup, "execute": after.Phases.Execute, "verify": after.Phases.Verify, "cleanup": after.Phases.Cleanup}
	for phase, baselineActions := range beforePhases {
		if len(afterPhases[phase]) < len(baselineActions) {
			return violation("action-removed", before.ID+":"+phase)
		}
		currentByID := map[string]Action{}
		for _, action := range afterPhases[phase] {
			currentByID[action.ID] = action
		}
		for _, old := range baselineActions {
			current, ok := currentByID[old.ID]
			if !ok {
				return violation("action-removed", before.ID+":"+old.ID)
			}
			if err := compareAction(before.ID, old, current); err != nil {
				return err
			}
		}
	}
	return nil
}

func compareAction(scenarioID string, before, after Action) error {
	if before.Type != after.Type || before.Service != after.Service || before.Method != after.Method || before.Path != after.Path || before.Left != after.Left || before.Right != after.Right || before.Op != after.Op || !reflect.DeepEqual(before.Body, after.Body) {
		return violation("action-regressed", scenarioID+":"+before.ID)
	}
	for key, value := range before.Headers {
		if after.Headers[key] != value {
			return violation("action-regressed", scenarioID+":"+before.ID+" Header")
		}
	}
	for key, value := range before.Capture {
		if after.Capture[key] != value {
			return violation("assertion-regressed", scenarioID+":"+before.ID+" Capture")
		}
	}
	if before.Expect == nil {
		return nil
	}
	if after.Expect == nil || before.Expect.Status != after.Expect.Status {
		return violation("assertion-regressed", scenarioID+":"+before.ID+" HTTP Status")
	}
	for _, expected := range before.Expect.JSON {
		matched := false
		for _, current := range after.Expect.JSON {
			if current.Path == expected.Path && current.Op == expected.Op && assertionValueAtLeast(expected, current) {
				matched = true
				break
			}
		}
		if !matched {
			return violation("assertion-regressed", scenarioID+":"+before.ID+":"+expected.Path)
		}
	}
	return nil
}

func assertionValueAtLeast(before, after JSONAssertion) bool {
	if before.Op != "gte" {
		return reflect.DeepEqual(before.Value, after.Value)
	}
	baseline, baselineOK := numericValue(before.Value)
	current, currentOK := numericValue(after.Value)
	return baselineOK && currentOK && current >= baseline
}

func numericValue(value any) (float64, bool) {
	switch item := value.(type) {
	case float64:
		return item, true
	case int:
		return float64(item), true
	case json.Number:
		parsed, err := strconv.ParseFloat(string(item), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func baselineExactPaths(root string, baseline NonRegressionBaseline) ([]string, error) {
	paths := append([]string(nil), baseline.ImmutablePaths...)
	for _, prefix := range baseline.ImmutablePrefixes {
		listed, err := gitList(root, baseline.SourceCommit, prefix)
		if err != nil {
			return nil, err
		}
		paths = append(paths, listed...)
	}
	unique := map[string]bool{}
	for _, path := range paths {
		unique[path] = true
	}
	paths = sortedKeys(unique)
	return paths, nil
}

func validateReplacement(root string, reader repositoryReader, migrations NonRegressionMigration, oldID, oldPath string) error {
	for _, mapping := range migrations.Mappings {
		if mapping.OldID != oldID || mapping.OldPath != oldPath {
			continue
		}
		if len(mapping.ReplacementIDs) == 0 || len(mapping.ReplacementPaths) == 0 || len(mapping.IntegrationProofs) < 2 || len(mapping.MigrationEvidence) < 1 {
			return violation("replacement-proof-insufficient", oldID)
		}
		for _, path := range mapping.ReplacementPaths {
			if path == oldPath {
				return violation("replacement-proof-insufficient", "Replacementは旧Pathから分離する必要があります")
			}
			if _, err := reader(path); err != nil {
				return violation("replacement-proof-insufficient", "Replacement Pathがありません: "+path)
			}
		}
		profiles := map[string]bool{}
		for _, proof := range mapping.IntegrationProofs {
			if err := validateNewEvidence(root, reader, proof, map[string]bool{"test-report": true, "conformance": true, "compatibility": true}); err != nil {
				return err
			}
			profiles[proof.Profile] = true
		}
		if !profiles["local"] || !profiles["container"] {
			return violation("replacement-proof-insufficient", "localとcontainerの統合Proofが必要です: "+oldID)
		}
		for _, proof := range mapping.MigrationEvidence {
			if err := validateNewEvidence(root, reader, proof, map[string]bool{"migration": true}); err != nil {
				return err
			}
		}
		return nil
	}
	return violation("replacement-mapping-missing", oldID)
}

func findReplacement(migrations NonRegressionMigration, oldID, oldPath string) *NonRegressionMapping {
	for index := range migrations.Mappings {
		mapping := &migrations.Mappings[index]
		if mapping.OldID == oldID && mapping.OldPath == oldPath {
			return mapping
		}
	}
	return nil
}

func isMissingReplacement(err error) bool {
	var item *NonRegressionViolation
	return errors.As(err, &item) && item.Code == "replacement-mapping-missing"
}

func intersects(left, right []string) bool {
	for _, item := range left {
		if contains(right, item) {
			return true
		}
	}
	return false
}

func validateNewEvidence(root string, reader repositoryReader, proof EvidenceRef, allowedKinds map[string]bool) error {
	data, err := reader(proof.Path)
	if err != nil || DigestBytes(data) != proof.Digest {
		return violation("replacement-proof-insufficient", "Evidence Digest不一致: "+proof.Path)
	}
	if _, err := gitFile(root, nonRegressionBaselineCommit, proof.Path); err == nil {
		return violation("replacement-proof-insufficient", "旧Evidenceの再利用は禁止です: "+proof.Path)
	}
	var record map[string]any
	if err := decodeStrictJSON(proof.Path, data, &record); err != nil {
		return err
	}
	if record["id"] != proof.ID || record["verdict"] != "pass" || record["profile"] != proof.Profile || !allowedKinds[fmt.Sprint(record["kind"])] {
		return violation("replacement-proof-insufficient", "Evidence種別またはVerdictが不正です: "+proof.ID)
	}
	return nil
}

var forbiddenControlPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)"(?:skip|disabled|optional)"\s*:\s*true`),
	regexp.MustCompile(`(?i)continue-on-error\s*:\s*true`),
	regexp.MustCompile(`(?i)(?:paths-ignore|branches-ignore)\s*:`),
	regexp.MustCompile(`(?im)^\s*if\s*:\s*(?:false|\$\{\{\s*false\s*\}\})\s*$`),
	regexp.MustCompile(`(?i)(?:t\.Skip|describe\.skip|it\.skip|test\.skip|@Disabled|pytest\.mark\.skip)\s*\(`),
}

func scanForbiddenControls(reader repositoryReader, paths []string) error {
	seen := map[string]bool{}
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		data, err := reader(path)
		if err != nil {
			continue
		}
		for _, pattern := range forbiddenControlPatterns {
			if pattern.Match(data) {
				return violation("forbidden-control", path)
			}
		}
	}
	return nil
}

func lineMultisetContains(current, baseline []byte) bool {
	counts := map[string]int{}
	for _, line := range strings.Split(string(current), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			counts[trimmed]++
		}
	}
	for _, line := range strings.Split(string(baseline), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		counts[trimmed]--
		if counts[trimmed] < 0 {
			return false
		}
	}
	return true
}

func ciStepSuperset(current, baseline []byte) bool {
	currentSteps := extractCISteps(current)
	baselineSteps := extractCISteps(baseline)
	for key, baselineStep := range baselineSteps {
		currentStep, ok := currentSteps[key]
		if !ok || !lineMultisetContains(currentStep, baselineStep) {
			return false
		}
		if !bytes.Contains(baselineStep, []byte("\n        if:")) && bytes.Contains(currentStep, []byte("\n        if:")) {
			return false
		}
	}
	return true
}

func extractCISteps(data []byte) map[string][]byte {
	lines := strings.Split(string(data), "\n")
	steps := map[string][]byte{}
	for index := 0; index < len(lines); index++ {
		line := lines[index]
		if !strings.HasPrefix(line, "      - name:") && !strings.HasPrefix(line, "      - uses:") {
			continue
		}
		key := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
		end := index + 1
		for end < len(lines) {
			candidate := lines[end]
			if strings.HasPrefix(candidate, "      - name:") || strings.HasPrefix(candidate, "      - uses:") || (candidate != "" && len(candidate)-len(strings.TrimLeft(candidate, " ")) < 6) {
				break
			}
			end++
		}
		steps[key] = []byte(strings.Join(lines[index:end], "\n"))
		index = end - 1
	}
	return steps
}

func isSuperset(current, baseline []string) bool {
	for _, item := range baseline {
		if !contains(current, item) {
			return false
		}
	}
	return true
}

func gitFile(root, commit, path string) ([]byte, error) {
	output, err := exec.Command("git", "-C", root, "show", commit+":"+filepath.ToSlash(path)).Output()
	if err != nil {
		return nil, err
	}
	return output, nil
}

func gitList(root, commit, prefix string) ([]string, error) {
	output, err := exec.Command("git", "-C", root, "ls-tree", "-r", "--name-only", commit, "--", prefix).Output()
	if err != nil {
		return nil, err
	}
	paths := []string{}
	for _, path := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if path != "" {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func decodeStrictJSON(path string, data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("%s は契約不適合です: %w", path, err)
	}
	if err := ensureEOF(decoder); err != nil {
		return fmt.Errorf("%s に余分なJSON値があります", path)
	}
	return nil
}

func violation(code, detail string) error {
	return &NonRegressionViolation{Code: code, Detail: detail}
}

func failNonRegression(report NonRegressionGateReport, err error) (NonRegressionGateReport, error) {
	report.Verdict = "fail"
	var item *NonRegressionViolation
	if errors.As(err, &item) {
		report.Checks = append(report.Checks, SelfAuditCheck{Name: item.Code, Verdict: "fail", Detail: item.Detail})
	}
	return report, fmt.Errorf("Interop Non-Regression Gate拒否: %w", err)
}
