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
    )
    if not all(required):
        raise SystemExit("Router Skillのfrontmatter、UI metadata、Reference Indexが不正です")
    print("検証済み: interoperability-router skill package")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
