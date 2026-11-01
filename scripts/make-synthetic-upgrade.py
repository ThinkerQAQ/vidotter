#!/usr/bin/env python3
"""Create fake next-version Qt update repo for isolated CI ONLY.

Never upload this synthetic version to production or GitHub releases.
"""
from pathlib import Path
import argparse
import json
import platform
import shutil
import subprocess

def main():
    p = argparse.ArgumentParser()
    p.add_argument("--stage", required=True, type=Path)
    p.add_argument("--tools-root", required=True, type=Path)
    p.add_argument("--workdir", required=True, type=Path)
    a = p.parse_args()
    a.workdir.mkdir(parents=True, exist_ok=True)
    copy = a.workdir / "synthetic-update-not-for-release"
    shutil.copytree(a.stage, copy)
    manifest = copy / "extension" / "manifest.json"
    data = json.loads(manifest.read_text(encoding="utf-8"))
    parts = data["version"].split(".")
    if len(parts) != 3 or not all(part.isdigit() for part in parts):
        raise RuntimeError("CI fixture requires a three-number semantic version")
    data["version"] = ".".join(parts[:2] + [str(int(parts[2]) + 1)])
    manifest.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    goos = {"Windows": "windows", "Linux": "linux", "Darwin": "darwin"}[platform.system()]
    arch = "arm64" if platform.machine().lower() in ("arm64", "aarch64") else "amd64"
    repogen_name = "repogen.exe" if goos == "windows" else "repogen"
    candidates = list(a.tools_root.rglob(repogen_name))
    if len(candidates) != 1:
        raise RuntimeError("Qt IFW repogen not found")
    output = a.workdir / "update-test-repo"
    subprocess.run(["go", "-C", "tools/vidotterdev", "run", ".", "update-repository",
        "--stage", str(copy), "--os", goos, "--arch", arch,
        "--repogen", str(candidates[0]), "--output", str(output)], check=True)
    assert (output / "Updates.xml").exists()
    import os
    if os.environ.get("GITHUB_ENV"):
        with open(os.environ["GITHUB_ENV"], "a", encoding="utf-8") as f:
            f.write(f"VIDOTTER_TEST_UPDATE_VERSION={data['version']}\n")
    print("Synthetic update repo (CI ONLY):", output, data["version"])

if __name__ == "__main__":
    main()
