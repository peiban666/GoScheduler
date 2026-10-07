import hashlib
from pathlib import Path
import tarfile
import tempfile
import unittest
import zipfile

from package_release import DOCUMENTS, package_binary, verify_binary, write_checksums


class ReleasePackagingTest(unittest.TestCase):
    def test_only_the_allowlisted_program_and_documents_are_archived(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = root / "source"
            assets = root / "assets"
            source.mkdir()
            assets.mkdir()
            for document in DOCUMENTS:
                (source / document).write_text(document, encoding="utf-8")
            (source / "app.ini").write_text("private fixture config", encoding="utf-8")
            binary = root / "program"
            binary.write_bytes(b"MZfixture")
            for platform in ("windows", "linux", "darwin"):
                archive = package_binary(binary, assets, "goscheduler", platform, "v1.6", source)
                if platform == "windows":
                    with zipfile.ZipFile(archive) as file:
                        names = file.namelist()
                else:
                    with tarfile.open(archive) as file:
                        names = file.getnames()
                        executable = file.getmember("goscheduler-" + platform + "-amd64/goscheduler")
                        self.assertEqual(executable.mode, 0o755)
                self.assertEqual(len(names), 5)
                self.assertFalse(any("app.ini" in name or "conf/" in name for name in names))
                self.assertTrue(any(name.endswith("/LICENSE") for name in names))

    def test_checksums_match_and_do_not_hash_themselves(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "binary").write_bytes(b"fixture")
            write_checksums(root)
            expected = hashlib.sha256(b"fixture").hexdigest() + "  binary\n"
            self.assertEqual((root / "SHA256SUMS.txt").read_text(), expected)
            write_checksums(root)
            self.assertEqual((root / "SHA256SUMS.txt").read_text(), expected)

    def test_binary_platform_magic_is_checked(self):
        with tempfile.TemporaryDirectory() as temporary:
            binary = Path(temporary) / "program"
            for platform, header in (("windows", b"MZfixture"),
                                     ("linux", b"\x7fELFfixture"),
                                     ("darwin", b"\xcf\xfa\xed\xfefixture")):
                binary.write_bytes(header)
                verify_binary(binary, platform)
            binary.write_bytes(b"invalid")
            with self.assertRaises(ValueError):
                verify_binary(binary, "linux")


if __name__ == "__main__":
    unittest.main()
