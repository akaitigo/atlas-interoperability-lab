package lab

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type NonRegressionMutationMatrix struct {
	SchemaVersion int                               `json:"schema_version"`
	Cases         []NonRegressionMutationMatrixCase `json:"cases"`
}

type NonRegressionMutationMatrixCase struct {
	ID           string `json:"id"`
	Operation    string `json:"operation"`
	ExpectedCode string `json:"expected_code"`
}

type NonRegressionMutationMatrixResult struct {
	SchemaVersion int                               `json:"schema_version"`
	Cases         []NonRegressionMutationCaseResult `json:"cases"`
	Verdict       string                            `json:"verdict"`
}

type NonRegressionMutationCaseResult struct {
	ID            string `json:"id"`
	Rejected      bool   `json:"rejected"`
	ViolationCode string `json:"violation_code"`
	Verdict       string `json:"verdict"`
}

type overlayRepository struct {
	root      string
	overrides map[string][]byte
	missing   map[string]bool
}

func (repository *overlayRepository) read(path string) ([]byte, error) {
	if repository.missing[path] {
		return nil, os.ErrNotExist
	}
	if data, ok := repository.overrides[path]; ok {
		return data, nil
	}
	return os.ReadFile(resolve(repository.root, path))
}

func RunNonRegressionMutationMatrix(root, matrixPath string) (NonRegressionMutationMatrixResult, error) {
	var matrix NonRegressionMutationMatrix
	if err := LoadJSON(resolve(root, matrixPath), &matrix); err != nil {
		return NonRegressionMutationMatrixResult{}, err
	}
	if matrix.SchemaVersion != 1 || len(matrix.Cases) == 0 {
		return NonRegressionMutationMatrixResult{}, fmt.Errorf("Non-Regression Mutation Matrix契約が不正です")
	}
	report := NonRegressionMutationMatrixResult{SchemaVersion: 1, Cases: []NonRegressionMutationCaseResult{}, Verdict: "pass"}
	for _, testCase := range matrix.Cases {
		repository := &overlayRepository{root: root, overrides: map[string][]byte{}, missing: map[string]bool{}}
		if err := applyNonRegressionMutation(repository, testCase.Operation); err != nil {
			return report, fmt.Errorf("Mutation %s: %w", testCase.ID, err)
		}
		_, gateErr := nonRegressionGate(root, repository.read)
		code := ""
		var rejected *NonRegressionViolation
		if errors.As(gateErr, &rejected) {
			code = rejected.Code
		}
		verdict := "pass"
		if gateErr == nil || code != testCase.ExpectedCode {
			verdict = "fail"
			report.Verdict = "fail"
		}
		report.Cases = append(report.Cases, NonRegressionMutationCaseResult{ID: testCase.ID, Rejected: gateErr != nil, ViolationCode: code, Verdict: verdict})
	}
	if report.Verdict != "pass" {
		return report, fmt.Errorf("Non-Regression Mutation Matrixがfailです")
	}
	return report, nil
}

func applyNonRegressionMutation(repository *overlayRepository, operation string) error {
	switch operation {
	case "delete-repository-contract":
		repository.missing["repo.yaml"] = true
	case "loosen-repository-boundary":
		data, err := repository.read("repo.yaml")
		if err != nil {
			return err
		}
		repository.overrides["repo.yaml"] = []byte(stringsReplaceOnce(string(data), "  cloud: denied\n", "  cloud: allowed\n"))
	case "delete-scenario":
		repository.missing["scenarios/failure.json"] = true
	case "scope-out-scenario", "aggregate-scenarios":
		return mutateJSON(repository, "compositions/fixture-stage2.json", func(document map[string]any) {
			scenarios, _ := document["scenarios"].([]any)
			filtered := []any{}
			for _, scenario := range scenarios {
				if scenario != "scenarios/recovery.json" {
					filtered = append(filtered, scenario)
				}
			}
			document["scenarios"] = filtered
		})
	case "shrink-assertion":
		return mutateScenarioAssertion(repository, "scenarios/normal.json", "verify", "trace-propagated", func(assertions []any) []any {
			return assertions[:len(assertions)-1]
		})
	case "weaken-threshold":
		return mutateScenarioAssertion(repository, "scenarios/failure.json", "verify", "failure-visible", func(assertions []any) []any {
			assertion, _ := assertions[0].(map[string]any)
			assertion["value"] = float64(1)
			return assertions
		})
	case "delete-component":
		return mutateJSON(repository, "compositions/fixture-stage2.json", func(document map[string]any) {
			subjects, _ := document["subjects"].([]any)
			document["subjects"] = subjects[:1]
		})
	case "change-version":
		return mutateJSON(repository, "compositions/fixture-stage2.json", func(document map[string]any) {
			subjects, _ := document["subjects"].([]any)
			sink, _ := subjects[1].(map[string]any)
			sink["version"] = "v0.9.0"
		})
	case "change-contract":
		data, err := repository.read("schemas/scenario.schema.json")
		if err != nil {
			return err
		}
		repository.overrides["schemas/scenario.schema.json"] = append(data, '\n')
	case "disable-test":
		data, err := repository.read("internal/lab/validate_test.go")
		if err != nil {
			return err
		}
		repository.overrides["internal/lab/validate_test.go"] = append(data, []byte("\n// t.Skip(\"disabled\")\n")...)
	case "disable-scenario":
		return mutateJSON(repository, "scenarios/failure.json", func(document map[string]any) {
			document["disabled"] = true
		})
	case "shrink-ci":
		data, err := repository.read(".github/workflows/ci.yml")
		if err != nil {
			return err
		}
		repository.overrides[".github/workflows/ci.yml"] = []byte(stringsReplaceOnce(string(data), "          go test ./...\n", ""))
	case "skip-ci-step":
		data, err := repository.read(".github/workflows/ci.yml")
		if err != nil {
			return err
		}
		repository.overrides[".github/workflows/ci.yml"] = []byte(stringsReplaceOnce(string(data), "      - name: Local E2E\n", "      - name: Local E2E\n        if: false\n"))
	case "replace-integration-with-mock":
		repository.overrides["internal/lab/runtime.go"] = []byte("package lab\n\n// static mock replacement\n")
	case "delete-failure-evidence":
		repository.missing["evidence/runs/container/failure.json"] = true
	case "delete-recovery-evidence":
		repository.missing["evidence/runs/local/recovery.json"] = true
	case "optionalize-component":
		return mutateJSON(repository, "compositions/fixture-stage2.json", func(document map[string]any) {
			subjects, _ := document["subjects"].([]any)
			sink, _ := subjects[1].(map[string]any)
			sink["optional"] = true
		})
	case "mapping-without-proof":
		repository.missing["scenarios/failure.json"] = true
		return mutateJSON(repository, "migrations/interop-v1-non-regression.json", func(document map[string]any) {
			document["mappings"] = []any{map[string]any{
				"old_id": "scenario:failure", "old_path": "scenarios/failure.json",
				"replacement_ids": []any{"failure-v2"}, "replacement_paths": []any{"scenarios/failure-v2.json"},
				"integration_proofs": []any{}, "migration_evidence": []any{},
			}}
		})
	case "mapping-single-profile":
		oldScenario, err := repository.read("scenarios/failure.json")
		if err != nil {
			return err
		}
		var replacementScenario map[string]any
		if err := json.Unmarshal(oldScenario, &replacementScenario); err != nil {
			return err
		}
		replacementScenario["id"] = "failure-v2"
		replacementData, err := json.MarshalIndent(replacementScenario, "", "  ")
		if err != nil {
			return err
		}
		repository.overrides["scenarios/failure-v2.json"] = append(replacementData, '\n')
		repository.missing["scenarios/failure.json"] = true
		if err := mutateJSON(repository, "compositions/fixture-stage2.json", func(document map[string]any) {
			scenarios, _ := document["scenarios"].([]any)
			for index, scenario := range scenarios {
				if scenario == "scenarios/failure.json" {
					scenarios[index] = "scenarios/failure-v2.json"
				}
			}
		}); err != nil {
			return err
		}
		integrationProofs := []any{}
		for index := 1; index <= 2; index++ {
			id := fmt.Sprintf("replacement.integration.%d", index)
			path := fmt.Sprintf("evidence/preview/replacement-integration-%d.json", index)
			data, err := json.MarshalIndent(map[string]any{"schema_version": 1, "id": id, "kind": "test-report", "profile": "local", "verdict": "pass"}, "", "  ")
			if err != nil {
				return err
			}
			data = append(data, '\n')
			repository.overrides[path] = data
			integrationProofs = append(integrationProofs, map[string]any{"id": id, "path": path, "digest": DigestBytes(data), "profile": "local"})
		}
		migrationPath := "evidence/preview/replacement-migration.json"
		migrationData, err := json.MarshalIndent(map[string]any{"schema_version": 1, "id": "replacement.migration.1", "kind": "migration", "profile": "local", "verdict": "pass"}, "", "  ")
		if err != nil {
			return err
		}
		migrationData = append(migrationData, '\n')
		repository.overrides[migrationPath] = migrationData
		return mutateJSON(repository, "migrations/interop-v1-non-regression.json", func(document map[string]any) {
			document["mappings"] = []any{map[string]any{
				"old_id": "scenario:failure", "old_path": "scenarios/failure.json",
				"replacement_ids": []any{"failure-v2"}, "replacement_paths": []any{"scenarios/failure-v2.json"},
				"integration_proofs": integrationProofs,
				"migration_evidence": []any{map[string]any{"id": "replacement.migration.1", "path": migrationPath, "digest": DigestBytes(migrationData), "profile": "local"}},
			}}
		})
	case "promotional-copy":
		data, err := repository.read("docs/CONTRACT.md")
		if err != nil {
			return err
		}
		repository.overrides["docs/CONTRACT.md"] = append(data, []byte("\nこのLabは世界一です。\n")...)
	case "author-praise":
		data, err := repository.read("README.md")
		if err != nil {
			return err
		}
		repository.overrides["README.md"] = append(data, []byte("\nakaitigo氏の優秀な成果です。\n")...)
	default:
		return fmt.Errorf("未知のMutationです: %s", operation)
	}
	return nil
}

func mutateJSON(repository *overlayRepository, path string, mutate func(map[string]any)) error {
	data, err := repository.read(path)
	if err != nil {
		return err
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return err
	}
	mutate(document)
	updated, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	repository.overrides[path] = append(updated, '\n')
	return nil
}

func mutateScenarioAssertion(repository *overlayRepository, path, phase, actionID string, mutate func([]any) []any) error {
	return mutateJSON(repository, path, func(document map[string]any) {
		phases, _ := document["phases"].(map[string]any)
		actions, _ := phases[phase].([]any)
		for _, raw := range actions {
			action, _ := raw.(map[string]any)
			if action["id"] != actionID {
				continue
			}
			expect, _ := action["expect"].(map[string]any)
			assertions, _ := expect["json"].([]any)
			expect["json"] = mutate(assertions)
		}
	})
}

func stringsReplaceOnce(value, old, replacement string) string {
	for index := 0; index+len(old) <= len(value); index++ {
		if value[index:index+len(old)] == old {
			return value[:index] + replacement + value[index+len(old):]
		}
	}
	return value
}
