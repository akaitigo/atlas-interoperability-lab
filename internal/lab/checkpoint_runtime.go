package lab

import (
	"fmt"
	"os"
)

type CheckpointRuntimeProfile struct {
	Profile              string `json:"profile"`
	Verdict              string `json:"verdict"`
	EvidenceSetDigest    string `json:"evidence_set_digest"`
	CleanupVerdict       string `json:"cleanup_verdict"`
	DiagnosticVerdict    string `json:"diagnostic_verdict"`
	RemainingProcesses   int    `json:"remaining_processes"`
	RemainingContainers  int    `json:"remaining_containers"`
	RemainingNetworks    int    `json:"remaining_networks"`
	RemainingImages      int    `json:"remaining_images"`
	CredentialsPersisted bool   `json:"credentials_persisted"`
}

type CheckpointRuntimeReport struct {
	SchemaVersion int                        `json:"schema_version"`
	Isolation     string                     `json:"isolation"`
	Profiles      []CheckpointRuntimeProfile `json:"profiles"`
	Reproducible  bool                       `json:"reproducible"`
	Verdict       string                     `json:"verdict"`
}

func ValidateIsolatedCheckpointRuntime(root string) (CheckpointRuntimeReport, error) {
	report := CheckpointRuntimeReport{SchemaVersion: 1, Isolation: "task-owned-temporary-copy", Profiles: []CheckpointRuntimeProfile{}, Verdict: "pass"}
	temporaryRoot, err := os.MkdirTemp("", "atlas-lab-checkpoint-runtime-")
	if err != nil {
		return report, err
	}
	defer os.RemoveAll(temporaryRoot)
	if err := copyRuntimeRepository(root, temporaryRoot); err != nil {
		return report, err
	}
	for _, profile := range []string{"local", "container"} {
		summary, err := Run(temporaryRoot, "compositions/fixture-stage2.json", profile)
		if err != nil {
			return report, fmt.Errorf("隔離Checkpoint %s E2Eが失敗しました: %w", profile, err)
		}
		if summary.Verdict != "pass" {
			return report, fmt.Errorf("隔離Checkpoint %s E2Eがpassではありません", profile)
		}
		var receipt CleanupReceipt
		if err := LoadJSON(resolve(temporaryRoot, summary.CleanupReceipt), &receipt); err != nil {
			return report, err
		}
		diagnosis := Diagnose(temporaryRoot, profile)
		profileReport := CheckpointRuntimeProfile{
			Profile: profile, Verdict: summary.Verdict, EvidenceSetDigest: summary.EvidenceSetDigest,
			CleanupVerdict: receipt.Verdict, DiagnosticVerdict: diagnosis.Verdict,
			RemainingProcesses: receipt.Processes, RemainingContainers: receipt.Containers,
			RemainingNetworks: receipt.Networks, RemainingImages: receipt.Images,
			CredentialsPersisted: receipt.CredentialsPersisted,
		}
		if receipt.Verdict != "pass" || diagnosis.Verdict != "pass" || receipt.Processes != 0 || receipt.Containers != 0 || receipt.Networks != 0 || receipt.Images != 0 || receipt.CredentialsPersisted {
			return report, fmt.Errorf("隔離Checkpoint %s Cleanupまたは診断がfailです", profile)
		}
		report.Profiles = append(report.Profiles, profileReport)
	}
	first, err := Run(temporaryRoot, "compositions/fixture-stage2.json", "local")
	if err != nil {
		return report, err
	}
	second, err := Run(temporaryRoot, "compositions/fixture-stage2.json", "local")
	if err != nil {
		return report, err
	}
	if first.EvidenceSetDigest != second.EvidenceSetDigest {
		return report, fmt.Errorf("隔離Checkpoint local Evidence Set Digestが再現しません")
	}
	report.Reproducible = true
	return report, nil
}
