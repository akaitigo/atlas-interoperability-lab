package lab

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const runtimeBindingMigrationPath = "migrations/runtime-binding-executable-attestation.json"

type RuntimeBindingMigration struct {
	SchemaVersion              int      `json:"schema_version"`
	ID                         string   `json:"id"`
	FromBindingState           string   `json:"from_binding_state"`
	ToBindingState             string   `json:"to_binding_state"`
	ClosedGap                  string   `json:"closed_gap"`
	PreservedGap               string   `json:"preserved_gap"`
	OldIDs                     []string `json:"old_ids"`
	ReplacementProofs          []string `json:"replacement_proofs"`
	RequiredNegativeAssertions []string `json:"required_negative_assertions"`
	Status                     string   `json:"status"`
}

func ValidateRuntimeBindingMigration(root string) error {
	return validateRuntimeBindingMigration(func(path string) ([]byte, error) {
		return os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	})
}

func validateRuntimeBindingMigration(reader repositoryReader) error {
	data, err := reader(runtimeBindingMigrationPath)
	if err != nil {
		return violation("runtime-binding-migration-removed", "Runtime Binding Migration Evidenceがありません")
	}
	var migration RuntimeBindingMigration
	if err := json.Unmarshal(data, &migration); err != nil {
		return violation("runtime-binding-migration-regressed", "Runtime Binding Migration Evidenceを解釈できません")
	}
	if migration.SchemaVersion != 1 || migration.ID != "runtime-binding-executable-attestation-v1" || migration.FromBindingState != "runtime-recipe-observed-with-explicit-gaps" || migration.ToBindingState != "runtime-executable-attested-with-certificate-gap" || migration.ClosedGap != "process-executable-attestation-unavailable" || migration.PreservedGap != "subject-v2-certificate-atomic-binding-unavailable" || migration.Status != "applied" {
		return violation("runtime-binding-migration-regressed", "Runtime Binding Migrationの状態対応が不正です")
	}
	if !sameSet(migration.OldIDs, []string{"local-runtime-binding", "container-runtime-binding"}) || !sameSet(migration.ReplacementProofs, []string{"evidence/preview/runtime-binding/local.binding.json", "evidence/preview/runtime-binding/container.binding.json"}) || !sameSet(migration.RequiredNegativeAssertions, []string{"executable-attestation-withdrawal", "executable-attestation-digest-drift"}) {
		return violation("runtime-binding-migration-regressed", fmt.Sprintf("Runtime Binding MigrationのProof closureが不正です: %s", migration.ID))
	}
	return nil
}
