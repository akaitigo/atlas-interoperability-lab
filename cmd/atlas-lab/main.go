package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/akaitigo/atlas-interoperability-lab/internal/lab"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	root, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		fatal(err)
	}
	switch os.Args[1] {
	case "validate":
		if err := lab.ValidateRepository(root); err != nil {
			fatal(err)
		}
		fmt.Println("VALID: Composition、Scenario、Release Lock、未完成Subject拒否契約")
	case "preflight":
		fs := flag.NewFlagSet("preflight", flag.ExitOnError)
		composition := fs.String("composition", "compositions/fixture-stage2.json", "")
		profile := fs.String("profile", "local", "")
		_ = fs.Parse(os.Args[2:])
		validated, err := lab.Preflight(root, *composition, *profile)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("PREFLIGHT PASS: %s %s %s\n", validated.Manifest.ID, *profile, validated.Digest)
	case "run":
		fs := flag.NewFlagSet("run", flag.ExitOnError)
		composition := fs.String("composition", "compositions/fixture-stage2.json", "")
		profile := fs.String("profile", "local", "")
		_ = fs.Parse(os.Args[2:])
		summary, err := lab.Run(root, *composition, *profile)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("E2E PASS: profile=%s evidence_set=%s cleanup=%s\n", summary.Profile, summary.EvidenceSetDigest, summary.CleanupReceipt)
	case "reproducibility":
		fs := flag.NewFlagSet("reproducibility", flag.ExitOnError)
		composition := fs.String("composition", "compositions/fixture-stage2.json", "")
		profile := fs.String("profile", "local", "")
		_ = fs.Parse(os.Args[2:])
		first, err := lab.Run(root, *composition, *profile)
		if err != nil {
			fatal(err)
		}
		second, err := lab.Run(root, *composition, *profile)
		if err != nil {
			fatal(err)
		}
		if first.EvidenceSetDigest != second.EvidenceSetDigest {
			fatal(fmt.Errorf("Evidence Set Digestが再現しません: first=%s second=%s", first.EvidenceSetDigest, second.EvidenceSetDigest))
		}
		fmt.Printf("REPRODUCIBLE: profile=%s evidence_set=%s\n", *profile, first.EvidenceSetDigest)
	case "publication-gate":
		report, err := lab.PublicationGate(root)
		if err != nil {
			fatal(err)
		}
		if err := lab.WriteJSON(filepath.Join(root, "evidence", "publication-gate.json"), report); err != nil {
			fatal(err)
		}
		fmt.Printf("PUBLICATION PASS: %d checks\n", len(report.Checks))
	case "provenance":
		if err := lab.GenerateProvenance(root); err != nil {
			fatal(err)
		}
		fmt.Println("PROVENANCE GENERATED: provenance.yaml")
	case "certificate":
		if err := lab.GenerateCertificate(root); err != nil {
			fatal(err)
		}
		fmt.Println("CERTIFICATE GENERATED: evidence/completion-certificate.json")
	case "certificate-check":
		if err := lab.ValidateCertificate(root); err != nil {
			fatal(err)
		}
		fmt.Println("CERTIFICATE VALID")
	case "diagnose":
		fs := flag.NewFlagSet("diagnose", flag.ExitOnError)
		profile := fs.String("profile", "local", "")
		_ = fs.Parse(os.Args[2:])
		printJSON(lab.Diagnose(root, *profile))
	case "self-audit":
		fs := flag.NewFlagSet("self-audit", flag.ExitOnError)
		allowDirty := fs.Bool("allow-dirty", false, "")
		_ = fs.Parse(os.Args[2:])
		report := lab.SelfAudit(root, *allowDirty)
		printJSON(report)
		if report.Verdict != "pass" {
			os.Exit(1)
		}
	case "definitive-gate":
		fs := flag.NewFlagSet("definitive-gate", flag.ExitOnError)
		composition := fs.String("composition", "compositions/fixture-stage2-v2-definitive.preview.json", "")
		asOf := fs.String("as-of", "2026-08-28T12:00:00Z", "")
		_ = fs.Parse(os.Args[2:])
		report, err := lab.EvaluateDefinitiveComposition(root, *composition, *asOf)
		if err != nil {
			fatal(err)
		}
		printJSON(report)
	case "definitive-matrix":
		fs := flag.NewFlagSet("definitive-matrix", flag.ExitOnError)
		matrix := fs.String("matrix", "tests/fixtures/definitive-gate-v2.matrix.json", "")
		_ = fs.Parse(os.Args[2:])
		report, err := lab.RunDefinitiveMatrix(root, *matrix)
		if writeErr := lab.WriteJSON(filepath.Join(root, "evidence", "preview", "definitive-gate-v2.matrix.json"), report); writeErr != nil {
			fatal(writeErr)
		}
		if err != nil {
			printJSON(report)
			fatal(err)
		}
		printJSON(report)
	case "definitive-migrate":
		fs := flag.NewFlagSet("definitive-migrate", flag.ExitOnError)
		composition := fs.String("composition", "compositions/fixture-stage2.json", "")
		_ = fs.Parse(os.Args[2:])
		report, err := lab.PlanV2Migration(root, *composition)
		if err != nil {
			fatal(err)
		}
		if err := lab.WriteJSON(filepath.Join(root, "evidence", "preview", "definitive-gate-v2.migration.json"), report); err != nil {
			fatal(err)
		}
		printJSON(report)
	case "definitive-preview-audit":
		report := lab.AuditDefinitivePreview(root)
		if err := lab.WriteJSON(filepath.Join(root, "evidence", "preview", "definitive-gate-v2.audit.json"), report); err != nil {
			fatal(err)
		}
		printJSON(report)
		if report.Verdict != "pass" {
			os.Exit(1)
		}
	case "legacy-v1-check":
		if err := lab.ValidateLegacyV1Bundle(root); err != nil {
			fatal(err)
		}
		printJSON(map[string]any{"schema_version": 1, "bundle": "fixture-http-stage2-v1", "core_policy_version": "1.0.0", "verdict": "pass"})
	default:
		usage()
		os.Exit(2)
	}
}
func usage() {
	fmt.Fprintln(os.Stderr, "usage: atlas-lab <validate|preflight|run|reproducibility|provenance|publication-gate|certificate|certificate-check|diagnose|self-audit|definitive-gate|definitive-matrix|definitive-migrate|definitive-preview-audit|legacy-v1-check>")
}
func fatal(err error) { fmt.Fprintln(os.Stderr, "ERROR:", err); os.Exit(1) }

func printJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		fatal(err)
	}
}
