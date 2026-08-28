package lab

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func Run(root, compositionPath, profile string) (RunSummary, error) {
	validated, err := Preflight(root, compositionPath, profile)
	if err != nil {
		return RunSummary{}, err
	}
	token, err := credential()
	if err != nil {
		return RunSummary{}, err
	}
	admin, err := credential()
	if err != nil {
		return RunSummary{}, err
	}
	runtime, err := startRuntime(root, profile, validated, token, admin)
	if err != nil {
		return RunSummary{}, err
	}
	variables := map[string]any{"token.valid": token, "token.admin": admin, "token.invalid": "invalid-fixture-token"}
	for name, url := range runtime.URLs() {
		variables[name+".url"] = url
	}
	reportPaths := []string{}
	reports := []ScenarioReport{}
	var runErr error
	for _, scenarioPath := range validated.Manifest.Scenarios {
		scenario, loadErr := ValidateScenario(root, scenarioPath, validated.Manifest.Axes)
		if loadErr != nil {
			runErr = loadErr
			break
		}
		scenarioDigest, _ := DigestFile(resolve(root, scenarioPath))
		report := executeScenario(scenario, profile, validated.Digest, scenarioDigest, runtime.URLs(), variables)
		path := filepath.Join(root, "evidence", "runs", profile, scenario.ID+".json")
		if err := WriteJSON(path, report); err != nil {
			runErr = err
			break
		}
		reportPaths = append(reportPaths, path)
		reports = append(reports, report)
		if report.Verdict != "pass" && runErr == nil {
			runErr = fmt.Errorf("Scenario %s が失敗しました", scenario.ID)
		}
	}
	receipt := runtime.Cleanup()
	receiptPath := filepath.Join(root, "cleanup", profile+".receipt.json")
	if err := WriteJSON(receiptPath, receipt); err != nil && runErr == nil {
		runErr = err
	}
	if receipt.Verdict != "pass" && runErr == nil {
		runErr = fmt.Errorf("%s ProfileのCleanupが不完全です", profile)
	}
	evidenceDigest, digestErr := DigestSet(reportPaths)
	if digestErr != nil && runErr == nil {
		runErr = digestErr
	}
	verdict := "pass"
	if runErr != nil {
		verdict = "fail"
	}
	summary := RunSummary{Profile: profile, CompositionDigest: validated.Digest, ScenarioReports: relativePaths(root, reportPaths), EvidenceSetDigest: evidenceDigest, Verdict: verdict, CleanupReceipt: filepath.ToSlash(mustRelative(root, receiptPath))}
	summaryPath := filepath.Join(root, "evidence", "runs", profile, "summary.json")
	if err := WriteJSON(summaryPath, summary); err != nil {
		return summary, err
	}
	if err := writeCoreEvidence(root, profile, validated, summaryPath, verdict); err != nil {
		return summary, err
	}
	return summary, runErr
}

func executeScenario(scenario Scenario, profile, compositionDigest, scenarioDigest string, urls map[string]string, variables map[string]any) ScenarioReport {
	report := ScenarioReport{SchemaVersion: 1, ScenarioID: scenario.ID, Profile: profile, CompositionDigest: compositionDigest, ScenarioDigest: scenarioDigest, Axes: scenario.Axes, Verdict: "pass"}
	mainActions := append(append([]Action{}, scenario.Phases.Setup...), scenario.Phases.Execute...)
	mainActions = append(mainActions, scenario.Phases.Verify...)
	failed := false
	for _, action := range mainActions {
		result := executeAction(action, urls, variables)
		report.Actions = append(report.Actions, result)
		if result.Verdict != "pass" {
			failed = true
			break
		}
	}
	for _, action := range scenario.Phases.Cleanup {
		result := executeAction(action, urls, variables)
		report.Actions = append(report.Actions, result)
		if result.Verdict != "pass" {
			failed = true
		}
	}
	if failed {
		report.Verdict = "fail"
	}
	return report
}

func executeAction(action Action, urls map[string]string, variables map[string]any) ActionResult {
	result := ActionResult{ID: action.ID, Type: action.Type, Verdict: "pass"}
	if action.Type == "compare" {
		left := resolveScalar(action.Left, variables)
		right := resolveScalar(action.Right, variables)
		result.Assertions = 1
		if !compareValues(left, right, action.Op) {
			result.Verdict = "fail"
			result.Error = fmt.Sprintf("compare %v %s %v", redact(left), action.Op, redact(right))
		}
		return result
	}
	base, ok := urls[action.Service]
	if !ok {
		result.Verdict = "fail"
		result.Error = "未知のService"
		return result
	}
	body := expand(action.Body, variables)
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			result.Verdict = "fail"
			result.Error = err.Error()
			return result
		}
		reader = bytes.NewReader(data)
	}
	path, _ := expandString(action.Path, variables).(string)
	req, err := http.NewRequest(action.Method, base+path, reader)
	if err != nil {
		result.Verdict = "fail"
		result.Error = err.Error()
		return result
	}
	for key, value := range action.Headers {
		expanded, _ := expandString(value, variables).(string)
		req.Header.Set(key, expanded)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	if err != nil {
		result.Verdict = "fail"
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()
	result.Status = resp.StatusCode
	var payload any
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, &payload); err != nil {
			result.Verdict = "fail"
			result.Error = "応答がJSONではありません"
			return result
		}
	}
	if action.Expect != nil {
		result.Assertions++
		if resp.StatusCode != action.Expect.Status {
			result.Verdict = "fail"
			result.Error = fmt.Sprintf("status expected=%d actual=%d", action.Expect.Status, resp.StatusCode)
			return result
		}
		for _, assertion := range action.Expect.JSON {
			result.Assertions++
			actual, exists := jsonPath(payload, assertion.Path)
			expected := expand(assertion.Value, variables)
			if !assertJSON(actual, exists, assertion.Op, expected) {
				result.Verdict = "fail"
				result.Error = fmt.Sprintf("json %s %s expected=%v actual=%v", assertion.Path, assertion.Op, expected, actual)
				return result
			}
		}
	}
	for name, path := range action.Capture {
		value, exists := jsonPath(payload, path)
		if !exists {
			result.Verdict = "fail"
			result.Error = "capture pathがありません: " + path
			return result
		}
		variables[name] = value
	}
	return result
}

func assertJSON(actual any, exists bool, op string, expected any) bool {
	switch op {
	case "exists":
		return exists
	case "eq":
		return exists && compareValues(actual, expected, "eq")
	case "gte":
		return exists && number(actual) >= number(expected)
	case "contains":
		if items, ok := actual.([]any); ok {
			for _, item := range items {
				if compareValues(item, expected, "eq") {
					return true
				}
			}
		}
		return false
	}
	return false
}
func compareValues(left, right any, op string) bool {
	switch op {
	case "eq":
		return fmt.Sprint(left) == fmt.Sprint(right)
	case "ne":
		return fmt.Sprint(left) != fmt.Sprint(right)
	case "gte":
		return number(left) >= number(right)
	}
	return false
}
func number(value any) float64 {
	switch item := value.(type) {
	case float64:
		return item
	case int:
		return float64(item)
	case string:
		parsed, _ := strconv.ParseFloat(item, 64)
		return parsed
	}
	return 0
}

func jsonPath(value any, path string) (any, bool) {
	if path == "" {
		return value, true
	}
	current := value
	for _, part := range strings.Split(path, ".") {
		switch item := current.(type) {
		case map[string]any:
			current, _ = item[part]
			if current == nil {
				return nil, false
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(item) {
				return nil, false
			}
			current = item[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func expand(value any, variables map[string]any) any {
	switch item := value.(type) {
	case string:
		return expandString(item, variables)
	case map[string]any:
		out := map[string]any{}
		for key, child := range item {
			out[key] = expand(child, variables)
		}
		return out
	case []any:
		out := make([]any, len(item))
		for i, child := range item {
			out[i] = expand(child, variables)
		}
		return out
	}
	return value
}
func expandString(value string, variables map[string]any) any {
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") && strings.Count(value, "${") == 1 {
		key := strings.TrimSuffix(strings.TrimPrefix(value, "${"), "}")
		if found, ok := variables[key]; ok {
			return found
		}
	}
	result := value
	for key, found := range variables {
		result = strings.ReplaceAll(result, "${"+key+"}", fmt.Sprint(found))
	}
	return result
}
func resolveScalar(value string, variables map[string]any) any { return expandString(value, variables) }
func redact(value any) any {
	text := fmt.Sprint(value)
	if len(text) > 20 {
		return "[redacted]"
	}
	return value
}

func credential() (string, error) {
	data := make([]byte, 24)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}
func relativePaths(root string, paths []string) []string {
	out := make([]string, len(paths))
	for i, path := range paths {
		out[i] = filepath.ToSlash(mustRelative(root, path))
	}
	return out
}
func mustRelative(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return relative
}

func writeCoreEvidence(root, profile string, validated ValidatedComposition, summaryPath, verdict string) error {
	environmentDigest, err := DigestFile(filepath.Join(root, "environments", profile+".json"))
	if err != nil {
		return err
	}
	sourceDigest, err := DigestFile(filepath.Join(root, "sources.lock.yaml"))
	if err != nil {
		return err
	}
	harnessPath, harnessDigest, err := WriteHarnessManifest(root)
	if err != nil {
		return err
	}
	artifactDigest, err := DigestFile(summaryPath)
	if err != nil {
		return err
	}
	info, err := os.Stat(summaryPath)
	if err != nil {
		return err
	}
	claims := []string{}
	for _, axis := range RequiredAxes {
		claims = append(claims, "interop."+axis)
	}
	record := map[string]any{"schema_version": 1, "id": "stage2." + profile + ".e2e", "atlas_id": "atlas-interoperability-lab", "claim_ids": claims, "kind": "test-report", "producer": "atlas-lab runner v1", "command": "go run ./cmd/atlas-lab run --composition compositions/fixture-stage2.json --profile " + profile, "created_at": time.Now().UTC().Format(time.RFC3339), "environment": map[string]any{"profile": profile, "manifest_digest": environmentDigest}, "source_digest": sourceDigest, "harness_digest": harnessDigest, "harness_path": filepath.ToSlash(mustRelative(root, harnessPath)), "artifact": map[string]any{"uri": filepath.ToSlash(mustRelative(root, summaryPath)), "digest": artifactDigest, "media_type": "application/vnd.atlas-lab.run-summary+json", "size_bytes": info.Size()}, "verdict": verdict, "retention": "git"}
	return WriteJSON(filepath.Join(root, "evidence", "records", "stage2."+profile+".evidence.json"), record)
}

func WriteHarnessManifest(root string) (string, string, error) {
	paths := []string{filepath.Join(root, "go.mod")}
	for _, dir := range []string{filepath.Join(root, "cmd", "atlas-lab"), filepath.Join(root, "internal", "lab")} {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".go") {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return "", "", err
		}
	}
	sort.Strings(paths)
	files := make([]map[string]string, 0, len(paths))
	for _, path := range paths {
		digest, err := DigestFile(path)
		if err != nil {
			return "", "", err
		}
		files = append(files, map[string]string{"path": filepath.ToSlash(mustRelative(root, path)), "digest": digest})
	}
	manifestPath := filepath.Join(root, "evidence", "harness-manifest.json")
	if err := WriteJSON(manifestPath, map[string]any{"schema_version": 1, "harness": "atlas-lab-runner-v1", "files": files}); err != nil {
		return "", "", err
	}
	digest, err := DigestFile(manifestPath)
	return manifestPath, digest, err
}
