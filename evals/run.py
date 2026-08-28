#!/usr/bin/env python3
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
ROUTER_PATH = ROOT / ".agents" / "skills" / "interoperability-router" / "scripts" / "route.py"
spec = importlib.util.spec_from_file_location("interoperability_router", ROUTER_PATH)
router = importlib.util.module_from_spec(spec)
spec.loader.exec_module(router)

cases_doc = json.loads((ROOT / "evals" / "cases.json").read_text(encoding="utf-8"))
results = []
for case in cases_doc["cases"]:
    actual = router.route(case["query"])
    passed = actual.get("decision") == case["decision"]
    if "reason" in case:
        passed = passed and actual.get("reason") == case["reason"]
    if "axes" in case:
        passed = passed and set(case["axes"]).issubset(set(actual.get("axes", [])))
    results.append({"id":case["id"],"category":case["category"],"result":"pass" if passed else "fail","assertion":case["assertion"]})
pass_rate = sum(item["result"] == "pass" for item in results) / len(results)
official = {"schema_version":1,"id":"interoperability.router-v1","atlas_id":"atlas-interoperability-lab","atlas_release":"v1.0.0","skill_id":"interoperability-router","generated_at":"2026-08-28T00:00:00+09:00","cases":results}
report = {"schema_version":1,"eval_id":"interoperability-router-v1","pass_rate":pass_rate,"minimum_pass_rate":cases_doc["minimum_pass_rate"],"verdict":"pass" if pass_rate >= cases_doc["minimum_pass_rate"] else "fail"}
(ROOT / "evals" / "skill").mkdir(exist_ok=True)
(ROOT / "evals" / "skill" / "interoperability-router.skill-eval.json").write_text(json.dumps(official, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8")
print(json.dumps(report, ensure_ascii=False, sort_keys=True))
raise SystemExit(0 if report["verdict"] == "pass" else 1)
