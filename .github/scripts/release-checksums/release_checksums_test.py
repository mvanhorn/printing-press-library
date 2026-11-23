import os
import subprocess
import tempfile
import unittest

from release_checksums import Source, checksums_text, parse_source, releases_missing_checksums, retained_assets, single_upload, tree_at


DIGEST_A = "sha256:" + "a" * 64
DIGEST_B = "sha256:" + "b" * 64
COMMIT = "c" * 40
TREE = "d" * 40


class ChecksumsTextTest(unittest.TestCase):
    def test_lists_every_asset_sorted_in_sha256sum_format(self):
        assets = [
            {"name": "x-pp-cli-linux-amd64", "digest": DIGEST_B},
            {"name": "x-pp-cli-darwin-arm64", "digest": DIGEST_A},
        ]
        self.assertEqual(
            checksums_text(assets),
            f"{'a' * 64}  x-pp-cli-darwin-arm64\n{'b' * 64}  x-pp-cli-linux-amd64\n",
        )

    def test_previous_checksums_file_is_not_listed(self):
        assets = [
            {"name": "checksums.txt", "digest": DIGEST_B},
            {"name": "x-pp-cli-linux-amd64", "digest": DIGEST_A},
        ]
        self.assertNotIn("checksums.txt", checksums_text(assets))

    def test_asset_without_sha256_digest_is_an_error(self):
        for digest in (None, "", "sha512:" + "a" * 128, "sha256:abc"):
            with self.subTest(digest=digest):
                with self.assertRaisesRegex(ValueError, "x-pp-cli-linux-amd64"):
                    checksums_text([{"name": "x-pp-cli-linux-amd64", "digest": digest}])

    def test_release_with_no_assets_is_an_error(self):
        with self.assertRaises(ValueError):
            checksums_text([{"name": "checksums.txt", "digest": DIGEST_A}])


class SourceLinesTest(unittest.TestCase):
    ASSETS = [{"name": "x-pp-cli-linux-amd64", "digest": DIGEST_A}]

    def test_source_lines_lead_as_comments(self):
        text = checksums_text(self.ASSETS, Source("library/cat/x", COMMIT, TREE))
        self.assertEqual(
            text,
            f"# source-commit {COMMIT}\n"
            f"# source-tree {TREE} library/cat/x\n"
            f"{'a' * 64}  x-pp-cli-linux-amd64\n",
        )

    def test_round_trips_through_parse_source(self):
        source = Source("library/cat/x", COMMIT, TREE)
        self.assertEqual(parse_source(checksums_text(self.ASSETS, source)), source)

    def test_file_without_source_lines_parses_as_none(self):
        self.assertIsNone(parse_source(checksums_text(self.ASSETS)))

    def test_rejects_values_that_are_not_object_ids(self):
        for commit, tree, directory in (
            ("main", TREE, "library/cat/x"),
            (COMMIT, "d" * 39, "library/cat/x"),
            (COMMIT, TREE, "library/cat/x y"),
            (COMMIT, TREE, ""),
        ):
            with self.subTest(commit=commit, tree=tree, directory=directory):
                with self.assertRaises(ValueError):
                    checksums_text(self.ASSETS, Source(directory, commit, tree))


class RetainedAssetsTest(unittest.TestCase):
    def test_lists_release_assets_this_build_did_not_upload(self):
        assets = [{"name": n} for n in ("checksums.txt", "x-pp-cli-darwin-arm64", "x-pp-cli-linux-amd64")]
        self.assertEqual(retained_assets(assets, {"x-pp-cli-darwin-arm64"}), ["x-pp-cli-linux-amd64"])
        self.assertEqual(retained_assets(assets, {"x-pp-cli-darwin-arm64", "x-pp-cli-linux-amd64"}), [])


class SingleUploadTest(unittest.TestCase):
    def test_assets_uploaded_together_count_as_one_build(self):
        assets = [
            {"name": "checksums.txt", "updated_at": "2026-10-01T00:00:00Z"},
            {"name": "x-pp-cli-darwin-arm64", "updated_at": "2026-10-28T18:00:00Z"},
            {"name": "x-pp-cli-linux-amd64", "updated_at": "2026-10-28T18:29:00Z"},
        ]
        self.assertTrue(single_upload(assets))

    def test_an_asset_from_an_earlier_upload_is_not_one_build(self):
        assets = [
            {"name": "x-pp-cli-darwin-arm64", "updated_at": "2026-10-28T18:00:00Z"},
            {"name": "x-pp-cli-linux-amd64", "updated_at": "2026-10-27T17:30:00Z"},
        ]
        self.assertFalse(single_upload(assets))

    def test_missing_timestamps_are_not_one_build(self):
        self.assertFalse(single_upload([{"name": "x-pp-cli-darwin-arm64"}]))
        self.assertFalse(single_upload([{"name": "checksums.txt", "updated_at": "2026-10-28T18:00:00Z"}]))


class TreeAtTest(unittest.TestCase):
    def test_reads_the_directory_tree_at_a_commit(self):
        with tempfile.TemporaryDirectory() as repo:
            env = {**os.environ, "GIT_CONFIG_GLOBAL": os.devnull, "GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t",
                   "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t"}

            def git(*args):
                return subprocess.run(["git", "-C", repo, *args], env=env, check=True, text=True,
                                      stdout=subprocess.PIPE).stdout.strip()

            git("init", "-q")
            os.makedirs(os.path.join(repo, "library/cat/x"))
            with open(os.path.join(repo, "library/cat/x/main.go"), "w") as f:
                f.write("package main\n")
            git("add", ".")
            git("commit", "-q", "-m", "release")
            commit = git("rev-parse", "HEAD")
            cwd = os.getcwd()
            os.chdir(repo)
            try:
                self.assertEqual(tree_at(commit, "library/cat/x"), git("rev-parse", "HEAD:library/cat/x"))
                self.assertIsNone(tree_at(commit, "library/cat/missing"))
            finally:
                os.chdir(cwd)


class ReleasesMissingChecksumsTest(unittest.TestCase):
    def test_selects_current_releases_without_checksums_file(self):
        releases = [
            {"tag_name": "a-current", "assets": [{"name": "a-pp-cli-linux-amd64"}]},
            {"tag_name": "b-current", "assets": [{"name": "checksums.txt"}]},
            {"tag_name": "npm-v0.1.24", "assets": [{"name": "pkg.tgz"}]},
        ]
        self.assertEqual(
            [r["tag_name"] for r in releases_missing_checksums(releases)],
            ["a-current"],
        )


if __name__ == "__main__":
    unittest.main()
