#!/usr/bin/env python3
"""CI-only orchestrator: verify a published release, then invoke the Go installer packager.

No installer created by this script is automatically published or advertised.
"""
import argparse
import hashlib
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

    tools_root = a.tools_root.resolve()
    run(sys.executable, "-m", "aqt", "install-tool", "-O", tools_root,
        qt_host, "desktop", "tools_ifw", "qt.tools.ifw.47")
    binary_name = "binarycreator.exe" if go_os == "windows" else "binarycreator"
    binaries = list(tools_root.rglob(binary_name))
    if len(binaries) != 1:
        raise RuntimeError(f"Qt IFW binarycreator not found uniquely: {binaries}")
    a.output_dir.mkdir(parents=True, exist_ok=True)
    extension = {"linux": ".run", "windows": ".exe", "darwin": ".dmg"}[go_os]
    artifact = a.output_dir.resolve() / f"VidOtter-Setup-{a.tag}-{name}-{arch}{extension}"
    run("go", "-C", "tools/vidotterdev", "run", ".", "installer",
        "--stage", stage, "--os", go_os, "--arch", arch,
        "--binarycreator", binaries[0], "--output", artifact)
    if not artifact.is_file() or artifact.stat().st_size == 0:
        raise RuntimeError("Qt IFW returned without an installer artifact")
    print(f"Installer candidate: {artifact} ({artifact.stat().st_size:,} bytes)")


if __name__ == "__main__":
    main()
