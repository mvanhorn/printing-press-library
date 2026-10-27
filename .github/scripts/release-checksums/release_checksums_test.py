import unittest

from release_checksums import checksums_text, releases_missing_checksums


DIGEST_A = "sha256:" + "a" * 64
DIGEST_B = "sha256:" + "b" * 64


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
