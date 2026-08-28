package lab

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Diagnosis struct {
	SchemaVersion int                 `json:"schema_version"`
	Profile       string              `json:"profile"`
	Verdict       string              `json:"verdict"`
	Findings      []DiagnosticFinding `json:"findings"`
}

type DiagnosticFinding struct {
	Code       string `json:"code"`
	ScenarioID string `json:"scenario_id,omitempty"`
	ActionID   string `json:"action_id,omitempty"`
	Message    string `json:"message"`
	NextAction string `json:"next_action"`
}

// Diagnose classifies only persisted observables. It deliberately excludes
// request bodies, headers and runtime credentials from its output.
func Diagnose(root, profile string) Diagnosis {
	diagnosis := Diagnosis{SchemaVersion: 1, Profile: profile, Verdict: "pass", Findings: []DiagnosticFinding{}}
	if profile != "local" && profile != "container" {
		return failedDiagnosis(diagnosis, DiagnosticFinding{
			Code:       "unsupported-profile",
			Message:    "診断対象Profileが宣言されていません。",
			NextAction: "Compositionに宣言されたlocalまたはcontainerを指定してください。",
		})
	}
	var summary RunSummary
	if err := LoadJSON(filepath.Join(root, "evidence", "runs", profile, "summary.json"), &summary); err != nil {
		return failedDiagnosis(diagnosis, DiagnosticFinding{
			Code:       "evidence-missing",
			Message:    "Run Summaryを検証できません。",
			NextAction: fmt.Sprintf("%s Profileを再実行し、Evidenceを生成してください。", profile),
		})
	}
	for _, reportPath := range summary.ScenarioReports {
		var report ScenarioReport
		if err := LoadJSON(filepath.Join(root, filepath.FromSlash(reportPath)), &report); err != nil {
			diagnosis = failedDiagnosis(diagnosis, DiagnosticFinding{
				Code:       "scenario-evidence-invalid",
				ScenarioID: strings.TrimSuffix(filepath.Base(reportPath), filepath.Ext(reportPath)),
				Message:    "Scenario Reportを検証できません。",
				NextAction: "対象Scenarioを再実行し、Reportの完全性を確認してください。",
			})
			continue
		}
		for _, action := range report.Actions {
			if action.Verdict == "pass" {
				continue
			}
			code, next := classifyActionFailure(action.Error)
			diagnosis = failedDiagnosis(diagnosis, DiagnosticFinding{
				Code:       code,
				ScenarioID: report.ScenarioID,
				ActionID:   action.ID,
				Message:    "保存済みObservableがOracleを満たしません。",
				NextAction: next,
			})
		}
	}
	var receipt CleanupReceipt
	if err := LoadJSON(filepath.Join(root, "cleanup", profile+".receipt.json"), &receipt); err != nil || receipt.Verdict != "pass" || receipt.CredentialsPersisted || receipt.Processes != 0 || receipt.Containers != 0 || receipt.Networks != 0 || receipt.Images != 0 {
		diagnosis = failedDiagnosis(diagnosis, DiagnosticFinding{
			Code:       "cleanup-incomplete",
			Message:    "Cleanup Receiptが完全回収を証明していません。",
			NextAction: "Run IDで隔離されたLab Resourceだけを確認し、他ProjectやDocker Volumeには触れないでください。",
		})
	}
	if diagnosis.Verdict == "pass" && summary.Verdict == "pass" {
		diagnosis.Findings = append(diagnosis.Findings, DiagnosticFinding{
			Code:       "run-healthy",
			Message:    "全ScenarioとCleanupの保存済みObservableはpassです。",
			NextAction: "追加対応は不要です。",
		})
	} else if summary.Verdict != "pass" && len(diagnosis.Findings) == 0 {
		diagnosis = failedDiagnosis(diagnosis, DiagnosticFinding{
			Code:       "summary-failed",
			Message:    "Run Summaryがfailです。",
			NextAction: "Scenario ReportとRunner標準エラーを照合してください。",
		})
	}
	return diagnosis
}

func failedDiagnosis(diagnosis Diagnosis, finding DiagnosticFinding) Diagnosis {
	diagnosis.Verdict = "fail"
	diagnosis.Findings = append(diagnosis.Findings, finding)
	return diagnosis
}

func classifyActionFailure(message string) (string, string) {
	normalized := strings.ToLower(message)
	switch {
	case strings.Contains(normalized, "status"):
		return "oracle-status-mismatch", "Subjectの到達性と期待HTTP StatusをScenario Oracleに照らして確認してください。"
	case strings.Contains(normalized, "json") || strings.Contains(normalized, "assert"):
		return "oracle-data-mismatch", "Response Schemaと固定ReleaseのCompatibility契約を確認してください。"
	case strings.Contains(normalized, "timeout") || strings.Contains(normalized, "connection") || strings.Contains(normalized, "network"):
		return "subject-unreachable", "対象SubjectのHealthと隔離Networkを確認し、同じCompositionで再実行してください。"
	case strings.Contains(normalized, "compare") || strings.Contains(normalized, "一致"):
		return "boundary-state-mismatch", "拒否前後の下流状態とFailure Propagation境界を確認してください。"
	default:
		return "oracle-mismatch", "該当ActionのOracleとSubject公開Interfaceを確認してください。"
	}
}
