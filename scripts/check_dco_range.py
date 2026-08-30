#!/usr/bin/env python3
import argparse
import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path

SHA = re.compile(r"^[0-9a-f]{40}$")


def git(repo: Path, *args: str, check: bool = True) -> subprocess.CompletedProcess:
    return subprocess.run(["git", "-C", str(repo), *args], check=check, stdout=subprocess.PIPE, stderr=subprocess.PIPE)


def check_range(repo: Path, base: str, head: str, core_script: Path) -> None:
    if git(repo, "rev-parse", "--is-shallow-repository").stdout.strip() != b"false":
        raise ValueError("shallow history is forbidden")
    if not SHA.fullmatch(base) or not SHA.fullmatch(head) or base == head:
        raise ValueError("non-empty explicit 40-hex commit range is required")
    for commit in (base, head):
        git(repo, "cat-file", "-e", commit + "^{commit}")
    if git(repo, "merge-base", "--is-ancestor", base, head, check=False).returncode != 0:
        raise ValueError("base must be an ancestor of head")
    count = int(git(repo, "rev-list", "--count", base + ".." + head).stdout)
    if count < 1:
        raise ValueError("commit range is empty")
    records = git(repo, "log", "--format=%H%x00%B%x00", base + ".." + head).stdout
    result = subprocess.run([sys.executable, str(core_script)], input=records, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if result.returncode != 0:
        raise ValueError("range contains a non-DCO commit: " + result.stderr.decode(errors="replace").strip())


def expect_failure(action, name: str) -> None:
    try:
        action()
    except Exception:
        return
    raise AssertionError(name + " negative fixture was accepted")


def self_test() -> None:
    with tempfile.TemporaryDirectory(prefix="atlas-lab-dco-range-") as directory:
        root = Path(directory)
        source = root / "source"
        source.mkdir()
        git(source, "init", "-q")
        git(source, "config", "user.name", "Fixture User")
        git(source, "config", "user.email", "fixture@example.invalid")
        (source / "proof.txt").write_text("one\n", encoding="utf-8")
        git(source, "add", "proof.txt")
        git(source, "commit", "-q", "-s", "-m", "test: signed fixture")
        base = git(source, "rev-parse", "HEAD").stdout.decode().strip()
        (source / "proof.txt").write_text("two\n", encoding="utf-8")
        git(source, "add", "proof.txt")
        git(source, "commit", "-q", "-s", "-m", "test: second signed fixture")
        (source / "proof.txt").write_text("three\n", encoding="utf-8")
        git(source, "add", "proof.txt")
        git(source, "commit", "-q", "-m", "test: unsigned fixture")
        head = git(source, "rev-parse", "HEAD").stdout.decode().strip()
        stub = root / "core_dco.py"
        stub.write_text("import sys\nparts=sys.stdin.buffer.read().split(b'\\0')\nbodies=parts[1::2]\nsys.exit(0 if bodies and all(b'Signed-off-by:' in body for body in bodies) else 1)\n", encoding="utf-8")
        expect_failure(lambda: check_range(source, "", head, stub), "missing-range")
        expect_failure(lambda: check_range(source, base, head, stub), "mixed-range-non-dco")
        shallow = root / "shallow"
        subprocess.run(["git", "clone", "-q", "--depth", "1", source.as_uri(), str(shallow)], check=True)
        expect_failure(lambda: check_range(shallow, "0" * 40, head, stub), "shallow-history")
    print("DCO RANGE NEGATIVE FIXTURES: pass (missing-range, mixed-range-non-dco, shallow-history)")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", default=".")
    parser.add_argument("--base")
    parser.add_argument("--head")
    parser.add_argument("--core-script")
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args()
    if args.self_test:
        self_test()
        return
    if not args.base or not args.head or not args.core_script:
        parser.error("--base, --head and --core-script are required")
    check_range(Path(args.repo).resolve(), args.base, args.head, Path(args.core_script).resolve())
    print(f"DCO RANGE: pass {args.base}..{args.head}")


if __name__ == "__main__":
    main()
