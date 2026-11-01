#!/usr/bin/env python3
"""CI-only orchestrator: verify a published release, then invoke the Go installer packager.

No installer created by this script is automatically published or advertised.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import platform
import shutil
import subprocess
import sys
import tarfile
import zipfile


def run(*args):
    print("+", " ".join(map(str, args)), flush=True)
    subprocess.run(list(map(str, args)), check=True)


def platform_info():
    system = platform.system()
    machine = platform.machine().lower()
    go_os, name, qt_host = {
        "Windows": ("windows", "windows", "windows"),
        "Linux": ("linux", "linux", "linux"),
        "Darwin": ("darwin", "macos", "mac"),
    }[system]
    arch = "arm64" if machine in ("aarch64", "arm64") else "amd64"
    return go_os, name, qt_host, arch


def secure_extract(archive, outdir, expected_root):
    def valid(name):
        posix = PurePosixPath(name.replace("\\", "/"))
        return posix.parts and posix.parts[0] == expected_root and ".." not in posix.parts
    outdir.mkdir(parents=True, exist_ok=True)
    if archive.suffix == ".zip":
        with zipfile.ZipFile(archive) as z:
            for member in z.infolist():
                if not valid(member.filename) or ((member.external_attr >> 16) & 0o170000) == 0o120000:
                    raise ValueError(f"Unsafe release ZIP entry: {member.filename}")
            z.extractall(outdir)
    else:
        with tarfile.open(archive, "r:gz") as t:
            for member in t.getmembers():
                if not valid(member.name) or not (member.isfile() or member.isdir()):
                    raise ValueError(f"Unsafe release tar entry: {member.name}")
            t.extractall(outdir, filter="data")
    return outdir / expected_root


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--tag", default="v1.6.2")
    parser.add_argument("--tools-root", type=Path, required=True)
    parser.add_argument("--working-dir", type=Path, required=True)
    parser.add_argument("--output-dir", type=Path, required=True)
    parser.add_argument("--local-release-dir", type=Path)
    parser.add_argument("--repo-only", action="store_true", help="Build only the update feed")
    parser.add_argument("--preview-source", action="store_true", help="Overlay current PR Bridge and extension on verified release for CI testing only")
    parser.add_argument("--update-base-url", default="", help="Already-published, verified HTTPS update base")
    a = parser.parse_args()

    go_os, name, qt_host, arch = platform_info()
    if not a.tag.startswith("v") or not all(s.isdigit() for s in a.tag[1:].split(".")):
        raise ValueError("Expected a release tag such as v1.6.2")
    base = f"vidotter-{a.tag}-{name}-{arch}"
    filename = base + (".zip" if go_os == "windows" else ".tar.gz")
    release_dir = a.working_dir.resolve() / "release"
    release_dir.mkdir(parents=True, exist_ok=True)
    if a.local_release_dir:
        for f in (filename, "SHA256SUMS.txt"):
            shutil.copy2(a.local_release_dir / f, release_dir / f)
    else:
        run("gh", "release", "download", a.tag, "--repo", "ThinkerQAQ/vidotter",
            "-D", release_dir, "-p", filename, "-p", "SHA256SUMS.txt")
    manifest = (release_dir / "SHA256SUMS.txt").read_text(encoding="utf-8")
    matches = [line.split()[0] for line in manifest.splitlines() if len(line.split()) >= 2
               and line.split()[1].lstrip("*") == filename]
    if len(matches) != 1 or len(matches[0]) != 64:
        raise ValueError("Release SHA-256 entry missing or ambiguous")
    checksum = hashlib.sha256()
    with (release_dir / filename).open("rb") as f:
        for chunk in iter(lambda: f.read(1048576), b""):
            checksum.update(chunk)
    if checksum.hexdigest() != matches[0].lower():
        raise ValueError("Release archive checksum mismatch")
    print("Release archive SHA-256 verified")
    stage = secure_extract(release_dir / filename, a.working_dir.resolve() / "extracted", base)
    if a.preview_source:
        if a.repo_only:
            raise ValueError("Preview source must never be published as a release update repository")
        suffix = ".exe" if go_os == "windows" else ""
        program = stage / "bin" / f"vidotter-{name}-{arch}{suffix}"
        run("go", "-C", "tools/vidotterdev", "run", ".", "build",
            "--os", go_os, "--arch", arch, "--output", program)
        shutil.copytree("extension", stage / "extension", dirs_exist_ok=True,
                        ignore=shutil.ignore_patterns("test", ".DS_Store"))
        # Preview packages used to advertise the production 1.6.2 manifest
        # despite having a new popup and worker. This caused Edge/Chrome to
        # keep an old MV3 service worker while reading the new popup files.
        # Give every CI preview a distinct 4-part Chrome-compatible version.
        manifest_path = stage / "extension" / "manifest.json"
        extension_manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        release_version = extension_manifest["version"]
        parts = release_version.split(".")
        if len(parts) != 3 or any(not part.isdigit() for part in parts):
            raise ValueError("PR preview requires a three-part release version")
        build_number = int(os.environ.get("GITHUB_RUN_NUMBER", "1"))
        if not 1 <= build_number < 65535:
            raise ValueError("Preview build number is outside Chrome's version segment range")
        extension_manifest["version"] = release_version + "." + str(build_number)
        extension_manifest["version_name"] = release_version + " PR preview #" + str(build_number)
        manifest_path.write_text(json.dumps(extension_manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        print("Preview extension version:", extension_manifest["version"])

        # PR tests build on the latest released media-dependency bundle. Do
        # not accidentally carry removed guides or stale README/license files
        # from that older release into the current preview installer.
        shutil.rmtree(stage / "docs", ignore_errors=True)
        for readme in ("README.md", "README_ZH.md", "LICENSE", "NOTICE", "THIRD_PARTY_NOTICES.md"):
            shutil.copy2(Path(readme), stage / readme)
        shutil.copytree("assets", stage / "assets", dirs_exist_ok=True)
        for helper in ("Setup-VidOtter-MCP.ps1", "setup-mcp.sh",
                       "Manage-VidOtter-MCP.ps1", "manage-mcp.sh"):
            shutil.copy2(Path("scripts") / helper, stage / "scripts" / helper)
        print("PR preview: Bridge and extension overlaid on official verified v1.6.2 dependency bundle")

    tools_root = a.tools_root.resolve()
    run(sys.executable, "-m", "aqt", "install-tool", "-O", tools_root,
        qt_host, "desktop", "tools_ifw", "qt.tools.ifw.47")
    a.output_dir.mkdir(parents=True, exist_ok=True)
    if not a.repo_only:
        binary_name = "binarycreator.exe" if go_os == "windows" else "binarycreator"
        binaries = list(tools_root.rglob(binary_name))
        if len(binaries) != 1:
            raise RuntimeError(f"Qt IFW binarycreator not found uniquely: {binaries}")
        extension = {"linux": ".run", "windows": ".exe", "darwin": ".dmg"}[go_os]
        artifact = a.output_dir.resolve() / f"VidOtter-Setup-{a.tag}-{name}-{arch}{extension}"
        args = ["go", "-C", "tools/vidotterdev", "run", ".", "installer",
                "--stage", str(stage), "--os", go_os, "--arch", arch,
                "--binarycreator", str(binaries[0]), "--output", str(artifact)]
        if a.update_base_url:
            if not a.update_base_url.startswith("https://"):
                raise ValueError("public update base must use HTTPS")
            args.extend(["--update-url", a.update_base_url.rstrip("/") + f"/updates/{name}/{arch}"])
        run(*args)
        if not artifact.is_file() or artifact.stat().st_size == 0:
            raise RuntimeError("Qt IFW returned without an installer artifact")
        print(f"Installer candidate: {artifact} ({artifact.stat().st_size:,} bytes)")
    repo_dir = a.output_dir.resolve() / "updates" / name / arch
    repogen_name = "repogen.exe" if go_os == "windows" else "repogen"
    repogen = list(tools_root.rglob(repogen_name))
    if len(repogen) != 1:
        raise RuntimeError(f"Qt IFW repogen not found uniquely: {repogen}")
    run("go", "-C", "tools/vidotterdev", "run", ".", "update-repository",
        "--stage", stage, "--os", go_os, "--arch", arch,
        "--repogen", repogen[0], "--output", repo_dir)
    if not (repo_dir / "Updates.xml").is_file():
        raise RuntimeError("Missing update repository metadata")
    print(f"Update repository candidate: {repo_dir}")


if __name__ == "__main__":
    main()
