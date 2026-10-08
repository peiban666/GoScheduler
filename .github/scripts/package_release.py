"""Build only tagged source and package an explicit, credential-free file list."""

import argparse
from datetime import datetime, timezone, timedelta
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tarfile
import zipfile


PLATFORMS = ("windows", "linux", "darwin")
PROGRAMS = {"goscheduler": "./cmd/goscheduler", "goscheduler-node": "./cmd/node"}
DOCUMENTS = ("LICENSE", "README.md", "CHANGELOG.md")
UPGRADE = """# 安装与保留数据升级

主程序已包含生产前端，无需另行安装 Node.js。数据库需独立部署。
首次使用请解压，运行 goscheduler web（Windows 为 goscheduler.exe web），
然后访问 http://localhost:5920 完成安装。任务节点运行 goscheduler-node；
HTTP 任务不依赖任务节点。

从旧版本升级：
1. 停止旧调度器并等待正在运行的任务结束。
2. 备份数据库、conf 目录及旧程序，确认备份可以恢复。
3. 保留原 conf/app.ini、conf/install.lock、conf/.version 和引用的证书。
4. 只替换对应平台的程序；保持原目录布局，使用原数据库和原表前缀。
5. 启动后检查任务、账号、节点、通知设置和下次执行时间。

不要同时运行新旧调度器，不要再次首次安装，不要用其他电脑的空库覆盖原库。
本包不包含数据库、账号、密码、Webhook 地址、加签密钥或本机运行配置。
macOS/Linux 直接下载裸文件时执行 chmod +x 程序名；压缩包内已带执行权限。
详细信息见 README.md 和 CHANGELOG.md。
"""


def verify_binary(path, platform):
    with path.open("rb") as stream:
        header = stream.read(4)
    expected = {"windows": header[:2] == b"MZ",
                "linux": header == b"\x7fELF",
                "darwin": header in (b"\xcf\xfa\xed\xfe", b"\xfe\xed\xfa\xcf")}
    if not expected[platform]:
        raise ValueError(f"Incorrect {platform} executable: {path.name}")


def package_binary(binary, assets, program, platform, tag, source):
    name = f"{program}-{tag}-{platform}-amd64"
    root = f"{program}-{platform}-amd64"
    executable = program + (".exe" if platform == "windows" else "")
    files = {executable: binary.read_bytes()}
    for document in DOCUMENTS:
        files[document] = (source / document).read_bytes()
    files["UPGRADE.md"] = UPGRADE.encode("utf-8")
    if platform == "windows":
        archive = assets / (name + ".zip")
        with zipfile.ZipFile(archive, "w", compression=zipfile.ZIP_DEFLATED) as output:
            for filename, content in files.items():
                output.writestr(f"{root}/{filename}", content)
        with zipfile.ZipFile(archive) as output:
            if output.testzip() is not None:
                raise ValueError(f"Archive integrity error: {archive.name}")
            actual = set(output.namelist())
    else:
        import io
        archive = assets / (name + ".tar.gz")
        with tarfile.open(archive, "w:gz") as output:
            for filename, content in files.items():
                entry = tarfile.TarInfo(f"{root}/{filename}")
                entry.size = len(content)
                entry.mode = 0o755 if filename == executable else 0o644
                output.addfile(entry, io.BytesIO(content))
        with tarfile.open(archive, "r:gz") as output:
            actual = set(output.getnames())
    if actual != {f"{root}/{filename}" for filename in files}:
        raise ValueError(f"Unexpected archive contents: {archive.name}")
    return archive


def write_checksums(assets):
    entries = []
    for path in sorted(assets.iterdir()):
        if path.is_file() and path.name != "SHA256SUMS.txt":
            entries.append(f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n")
    (assets / "SHA256SUMS.txt").write_text("".join(entries), encoding="utf-8")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--tag", required=True)
    parser.add_argument("--commit", required=True)
    args = parser.parse_args()
    if not re.fullmatch(r"v\d+\.\d+(?:\.\d+)?", args.tag):
        parser.error("Expected a version tag such as v1.6")
    if not re.fullmatch(r"[0-9a-f]{40}", args.commit):
        parser.error("Expected the full source commit SHA")
    source = args.source.resolve()
    output = args.output.resolve()
    if output == source or output in source.parents:
        parser.error("Output cannot replace the source tree")
    actual = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=source, text=True).strip()
    if actual != args.commit:
        parser.error("Source checkout differs from the requested release commit")
    assets = output / "assets"
    if assets.exists():
        parser.error("Use a fresh output directory; existing release files are not overwritten")
    assets.mkdir(parents=True)
    version = args.tag.removeprefix("v")
    date = datetime.now(timezone(timedelta(hours=8))).strftime("%Y-%m-%dT%H:%M:%S%z")
    ldflags = f"-s -w -X main.AppVersion={version} -X main.GitCommit={args.commit[:12]} -X main.BuildDate={date}"
    for platform in PLATFORMS:
        for program, command in PROGRAMS.items():
            suffix = ".exe" if platform == "windows" else ""
            binary = assets / f"{program}-{args.tag}-{platform}-amd64{suffix}"
            env = {**os.environ, "CGO_ENABLED": "0", "GOOS": platform, "GOARCH": "amd64"}
            subprocess.run(["go", "build", "-trimpath", "-ldflags", ldflags,
                            "-o", str(binary), command], cwd=source, env=env, check=True)
            binary.chmod(0o755)
            verify_binary(binary, platform)
            package_binary(binary, assets, program, platform, args.tag, source)
    info = {"tag": args.tag, "source_commit": args.commit, "built_at": date,
            "timezone": "Asia/Shanghai", "go": subprocess.check_output(["go", "version"], text=True).strip(),
            "targets": [f"{platform}/amd64" for platform in PLATFORMS],
            "frontend": "production frontend rebuilt, tested and embedded by GitHub Actions"}
    lockfile = source / "web/vue/package-lock.json"
    if lockfile.exists():
        lock = json.loads(lockfile.read_text(encoding="utf-8"))
        info["frontend_lockfile_sha256"] = hashlib.sha256(lockfile.read_bytes()).hexdigest()
        info["frontend_dependencies"] = {
            package: lock["packages"]["node_modules/" + package]["version"]
            for package in ("vue", "vue-template-compiler", "element-ui", "webpack")
        }
    if os.environ.get("RELEASE_TOOLING_COMMIT"):
        info["release_tooling_commit"] = os.environ["RELEASE_TOOLING_COMMIT"]
    (assets / "BUILD-INFO.json").write_text(json.dumps(info, indent=2) + "\n", encoding="utf-8")
    write_checksums(assets)
    changelog = (source / "CHANGELOG.md").read_text(encoding="utf-8")
    intro = f"""# GoScheduler {version}

由 GitHub Actions 从 `{args.tag}`（提交 `{args.commit}`）自动测试、构建和发布。
下载对应平台的 `goscheduler` 主程序；Shell 任务另外下载 `goscheduler-node`。
Windows 使用 `.exe` 或 `.zip`，Linux/macOS 使用裸程序或 `.tar.gz`。
全部压缩包含 MIT 许可证、更新日志和保留数据的升级说明。
`SHA256SUMS.txt` 提供文件校验，`BUILD-INFO.json` 记录构建来源。

**升级前备份原数据库和 conf 目录，仅替换程序，不覆盖原数据。**

"""
    (output / "RELEASE-NOTES.md").write_text(intro + changelog, encoding="utf-8")
    print(f"Verified {len(list(assets.iterdir()))} release assets for {args.tag}")


if __name__ == "__main__":
    main()
