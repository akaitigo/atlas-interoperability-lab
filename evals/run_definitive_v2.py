#!/usr/bin/env python3
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
ROUTER_PATH = ROOT / ".agents" / "skills" / "interoperability-router" / "scripts" / "route.py"
spec = importlib.util.spec_from_file_location("interoperability_router_v2", ROUTER_PATH)
router = importlib.util.module_from_spec(spec)
spec.loader.exec_module(router)

document = json.loads((ROOT / "evals" / "definitive-gate-v2.cases.json").read_text(encoding="utf-8"))
results = []
for case in document["cases"]:
    actual = router.route(case["query"])
    checked = {key: actual.get(key) == value for key, value in case.items() if key not in {"id", "query"}}
    results.append({"id": case["id"], "result": "pass" if all(checked.values()) else "fail", "checks": checked})
pass_rate = sum(item["result"] == "pass" for item in results) / len(results)
report = {
    "schema_version": 1,
    "id": "interoperability-router-definitive-v2-preview",
    "core_v2_status": "draft",
    "pass_rate": pass_rate,
    "cases": results,
    "verdict": "pass" if pass_rate >= document["minimum_pass_rate"] else "fail"
}
output = ROOT / "evals" / "preview" / "interoperability-router.definitive-v2-preview.json"
output.parent.mkdir(exist_ok=True)
output.write_text(json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8")
print(json.dumps({"pass_rate": pass_rate, "verdict": report["verdict"]}, ensure_ascii=False, sort_keys=True))
raise SystemExit(0 if report["verdict"] == "pass" else 1)
