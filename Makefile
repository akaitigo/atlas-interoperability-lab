CORE_DIR ?= ../reference-atlas-core
CORE_COMMIT := cf9e6e2d981305c83f970c1f21a1ddc9c1109263
CORE_MAIN_COMMIT := 072d7ca77981f51754e824d70c6d4ecd55ea67e5
FE_DIR ?= ../frontend-behavior-atlas
FE_DEPTH_COMMIT := deadad18b6588d2c907170a451c3b5cea5ea4192
GO_CACHE_DIR := $(CURDIR)/.cache/go-build

.PHONY: test depth-reference core-main-reference lab-validate graph-check evidence-local evidence-container evidence reproducibility skill-validate skill-eval skill-eval-v2 non-regression evidence-dependency-matrix composition-compatibility-matrix preview-publication definitive-preview legacy-v1-check diagnose provenance publication certificate core-validate core-audit dco-check self-audit cleanup-check check

depth-reference:
	git -C "$(FE_DIR)" cat-file -e "$(FE_DEPTH_COMMIT)^{commit}"

core-main-reference:
	git -C "$(CORE_DIR)" cat-file -e "$(CORE_MAIN_COMMIT)^{commit}"

test: depth-reference core-main-reference
	GOCACHE="$(GO_CACHE_DIR)" go test ./...

lab-validate:
	git -C "$(CORE_DIR)" cat-file -e "$(CORE_COMMIT)^{commit}"
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

skill-eval-v2:
	python3 evals/run_definitive_v2.py

legacy-v1-check:
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab legacy-v1-check

non-regression:
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab non-regression-gate

evidence-dependency-matrix: core-main-reference
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab evidence-dependency-matrix

composition-compatibility-matrix: core-main-reference depth-reference
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab composition-compatibility-matrix

preview-publication: core-main-reference depth-reference
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab preview-publication-gate

definitive-preview: depth-reference core-main-reference non-regression legacy-v1-check skill-eval-v2 evidence-dependency-matrix composition-compatibility-matrix
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab depth-parity
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab definitive-matrix
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab definitive-migrate
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab definitive-preview-audit
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab preview-publication-gate

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
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab legacy-v1-check

core-audit:
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab legacy-v1-check

dco-check:
	git log --format='%H%x00%B%x00' | python3 "$(CORE_DIR)/scripts/check_dco.py"

self-audit:
	GOCACHE="$(GO_CACHE_DIR)" go run ./cmd/atlas-lab self-audit

cleanup-check:
	test '"verdict": "pass"' = "$$(rg -o '"verdict": "pass"' cleanup/local.receipt.json)"
	test '"verdict": "pass"' = "$$(rg -o '"verdict": "pass"' cleanup/container.receipt.json)"

check: non-regression test graph-check skill-validate skill-eval evidence reproducibility cleanup-check diagnose provenance publication certificate core-validate core-audit dco-check
