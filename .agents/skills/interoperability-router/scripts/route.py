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
    matches = []
    for item in index["routes"]:
        if any(keyword.casefold() in normalized for keyword in item["keywords"]):
            matches.append({"axis":item["axis"],"scenarios":item["scenarios"]})
    if not matches:
        return {"decision":"coverage_gap","reason":"no_declared_axis","path":index["rejections"]["coverage_gap"]}
    scenarios = sorted({path for item in matches for path in item["scenarios"]})
    return {"decision":"route","composition":index["composition"],"axes":[item["axis"] for item in matches],"scenarios":scenarios,"evidence":index["evidence"]}

def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--query", required=True)
    args = parser.parse_args()
    print(json.dumps(route(args.query), ensure_ascii=False, sort_keys=True))

if __name__ == "__main__":
    main()
