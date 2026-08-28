#!/usr/bin/env python3
import argparse
import json
from pathlib import Path

SKILL_ROOT = Path(__file__).resolve().parents[1]

def route(query: str) -> dict:
    index = json.loads((SKILL_ROOT / "references" / "index.json").read_text(encoding="utf-8"))
    normalized = query.casefold()
    if "未完成" in normalized or "incomplete" in normalized:
        return {"decision":"reject","reason":"incomplete_subject","path":index["rejections"]["incomplete_subject"]}
    preview = index["definitive_gate_v2"]
    non_regression = index["non_regression"]["command"]
    if any(keyword in normalized for keyword in ("非後退", "non-regression", "不足を隠", "scenario削除", "シナリオ削除", "ci縮小", "skip", "disabled", "optional化", "mock置換")):
        return {"decision":"non_regression_gate","command":non_regression,"baseline":index["non_regression"]["baseline"]}
    if any(keyword in normalized for keyword in ("evidence dependency", "dependency graph", "stale", "digest-only", "digest only", "再実行漏れ", "output退避", "closure構造", "proof構造", "consumer互換")):
        dependency = index["evidence_dependency"]
        return {"decision":"evidence_dependency","status":"main-ci-confirmed","command":dependency["command"],"core_lock":dependency["core_lock"],"matrix":dependency["matrix"],"warning":dependency["warning"],"non_regression_command":non_regression}
    if any(keyword in normalized for keyword in ("depth", "parity", "18軸", "深度", "surface", "pattern proof", "850", "421", "統合trace", "個別proof")):
        depth = index["depth_reference"]
        scenario_boundary = any(keyword in normalized for keyword in ("surface", "pattern proof", "850", "421", "統合trace", "個別proof"))
        return {"decision":"depth_parity","status":"incomplete","command":depth["command"],"reference":depth["lock"],"scenario_contract":depth["scenario_contract"],"subject_depth_parity":depth["subject_depth_parity"],"integration_proofs":depth["integration_proofs"],"warning":"integrated-trace-not-component-proof" if scenario_boundary else "integration-proof-cannot-promote-depth-gap","non_regression_command":non_regression}
    if "旧v1" in normalized or "legacy v1" in normalized or "v1 bundle" in normalized:
        return {"decision":"legacy_v1","status":"verifiable","command":preview["legacy_v1_command"],"non_regression_command":non_regression}
    if "移行" in normalized or "migration" in normalized:
        return {"decision":"migration_preview","status":preview["status"],"command":preview["migration_command"],"warning":"no-definitive-promotion","non_regression_command":non_regression}
    if any(keyword in normalized for keyword in ("definitive", "決定版", "bounded", "epoch-complete", "v2", "excluded", "infeasible", "partial", "混在", "失効", "更新")):
        bounded = "bounded" in normalized or "epoch-complete" in normalized
        matrix = any(keyword in normalized for keyword in ("excluded", "infeasible", "partial", "混在", "失効", "更新"))
        return {
            "decision":"bounded_preview" if bounded else "definitive_preview",
            "status":preview["status"],
            "composition":preview["bounded_composition"] if bounded else preview["definitive_composition"],
            "command":preview["matrix_command"] if matrix else (preview["bounded_command"] if bounded else preview["definitive_command"]),
            "warning":"core-v2-draft",
            "non_regression_command":non_regression
        }
    matches = []
    for item in index["routes"]:
        if any(keyword.casefold() in normalized for keyword in item["keywords"]):
            matches.append({"axis":item["axis"],"scenarios":item["scenarios"]})
    if not matches:
        return {"decision":"coverage_gap","reason":"no_declared_axis","path":index["rejections"]["coverage_gap"]}
    scenarios = sorted({path for item in matches for path in item["scenarios"]})
    result = {"decision":"route","composition":index["composition"],"axes":[item["axis"] for item in matches],"scenarios":scenarios,"evidence":index["evidence"],"claim_evidence_graph":index["claim_evidence_graph"],"self_audit_command":index["self_audit_command"],"non_regression_command":non_regression}
    if any(keyword in normalized for keyword in ("失敗", "障害", "failure", "diagnos", "診断")):
        profile = "container" if "docker" in normalized or "container" in normalized else "local"
        result["diagnostic_command"] = index["diagnostics"][profile]
    return result

def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--query", required=True)
    args = parser.parse_args()
    print(json.dumps(route(args.query), ensure_ascii=False, sort_keys=True))

if __name__ == "__main__":
    main()
