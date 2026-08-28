CORE_DIR ?= ../reference-atlas-core
CORE_COMMIT := cf9e6e2d981305c83f970c1f21a1ddc9c1109263
GO_CACHE_DIR := $(CURDIR)/.cache/go-build

.PHONY: test lab-validate graph-check evidence-local evidence-container evidence reproducibility skill-validate skill-eval diagnose provenance publication certificate core-validate core-audit dco-check self-audit cleanup-check check

test:
	GOCACHE="$(GO_CACHE_DIR)" go test ./...

lab-validate:
	test "$$(git -C "$(CORE_DIR)" rev-parse main)" = "$(CORE_COMMIT)"
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab validate

graph-check: lab-validate

evidence-local:
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab run --composition compositions/fixture-stage2.json --profile local

evidence-container:
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab run --composition compositions/fixture-stage2.json --profile container

evidence: evidence-local evidence-container

reproducibility:
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab reproducibility --composition compositions/fixture-stage2.json --profile local

skill-validate:
	python3 scripts/validate_skill.py

skill-eval:
	python3 evals/run.py

diagnose:
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab diagnose --profile local
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab diagnose --profile container

publication:
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab publication-gate

provenance:
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab provenance

certificate:
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab certificate
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab certificate-check

core-validate:
	GOCACHE="$(GO_CACHE_DIR)" go -C "$(CORE_DIR)" run ./cmd/atlas validate "$(CURDIR)/atlas.yaml" "$(CURDIR)/mastery.yaml" "$(CURDIR)/coverage.yaml" "$(CURDIR)/sources.lock.yaml" "$(CURDIR)/skill.package.yaml" "$(CURDIR)/provenance.yaml" "$(CURDIR)/third_party/manifest.yaml" "$(CURDIR)"/claims/*.claim.json "$(CURDIR)"/evidence/records/*.evidence.json "$(CURDIR)"/evals/skill/*.skill-eval.json "$(CURDIR)/evidence/completion-certificate.json"

core-audit:
	GOCACHE="$(GO_CACHE_DIR)" go -C "$(CORE_DIR)" run ./cmd/atlas audit "$(CURDIR)"

dco-check:
	git log --format='%H%x00%B%x00' | python3 "$(CORE_DIR)/scripts/check_dco.py"

self-audit:
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab self-audit

cleanup-check:
	test '"verdict": "pass"' = "$$(rg -o '"verdict": "pass"' cleanup/local.receipt.json)"
	test '"verdict": "pass"' = "$$(rg -o '"verdict": "pass"' cleanup/container.receipt.json)"

check: test graph-check skill-validate skill-eval evidence reproducibility cleanup-check diagnose provenance publication certificate core-validate core-audit dco-check
