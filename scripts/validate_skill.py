#!/usr/bin/env python3
"""Repository内Router Skillのportableな最低契約を検証する。"""

import json
from pathlib import Path


def main() -> int:
    root = Path(__file__).resolve().parents[1]
    skill_root = root / ".agents" / "skills" / "interoperability-router"
    skill = (skill_root / "SKILL.md").read_text(encoding="utf-8")
    metadata = (skill_root / "agents" / "openai.yaml").read_text(encoding="utf-8")
    index = json.loads((skill_root / "references" / "index.json").read_text(encoding="utf-8"))
    required = (
        skill.startswith("---\nname: interoperability-router\n"),
        "description:" in skill,
        "[TODO" not in skill,
        "$interoperability-router" in metadata,
        index.get("composition") == "compositions/fixture-stage2.json",
        len(index.get("routes", [])) == 10,
        index.get("claim_evidence_graph") == "graphs/fixture-stage2.claim-evidence.json",
        set(index.get("diagnostics", {})) == {"local", "container"},
        index.get("self_audit_command") == "go run ./cmd/atlas-lab self-audit",
        index.get("definitive_gate_v2", {}).get("status") == "draft",
        index.get("definitive_gate_v2", {}).get("legacy_v1_command") == "go run ./cmd/atlas-lab legacy-v1-check",
        index.get("non_regression", {}).get("command") == "go run ./cmd/atlas-lab non-regression-gate",
        index.get("depth_reference", {}).get("lock") == "depth/fe-depth-reference.lock.json",
        index.get("depth_reference", {}).get("scenario_contract") == "depth/fe-scenario-contract.lock.json",
        index.get("depth_reference", {}).get("command") == "go run ./cmd/atlas-lab depth-parity",
        index.get("evidence_dependency", {}).get("core_lock") == "compatibility/evidence-dependency-core.lock.json",
        index.get("evidence_dependency", {}).get("command") == "go run ./cmd/atlas-lab evidence-dependency-matrix",
    )
    if not all(required):
        raise SystemExit("Router Skillのfrontmatter、UI metadata、Reference Indexが不正です")
    print("検証済み: interoperability-router skill package")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
