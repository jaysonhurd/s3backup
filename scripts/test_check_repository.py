"""Exercise repository checks against isolated Git repositories."""

import subprocess
import tempfile
import unittest
from pathlib import Path

from check_repository import check


class RepositoryChecks(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        subprocess.run(["git", "init", "--quiet", str(self.root)], check=True)

    def write(self, name, text):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)

    def test_valid_repository_and_fenced_examples(self):
        self.write("skills/example/SKILL.md", "---\nname: example\ndescription: Example workflow.\n---\n")
        self.write("guide.md", '# Guide\n\n## Usage\n\n<a id="custom"></a>\n')
        self.write("README.md", "[Guide](guide.md#usage)\n[Custom](guide.md#custom)\n"
                   "[External](https://example.com/not-checked)\n"
                   "```markdown\n[Example](missing.md)\n```\n")
        self.write("install.sh", "#!/usr/bin/env bash\nexit 0\n")
        self.assertEqual(check(self.root), [])

    def test_missing_link_and_anchor(self):
        self.write("guide.md", "# Guide\n")
        self.write("README.md", "[Missing](missing.md)\n[Anchor](guide.md#missing)\n")
        errors = check(self.root)
        self.assertEqual(len(errors), 2)
        self.assertTrue(any("missing local link" in error for error in errors))
        self.assertTrue(any("missing Markdown anchor" in error for error in errors))

    def test_duplicate_heading_anchors(self):
        self.write("README.md", "# Title\n\n## Usage\n\n## Usage\n\n[Second](#usage-1)\n")
        self.assertEqual(check(self.root), [])

    def test_invalid_skill_metadata(self):
        self.write("skills/example/SKILL.md", "---\nname: Wrong_Name\ndescription: ''\n---\n")
        errors = check(self.root)
        self.assertTrue(any("invalid skill name" in error for error in errors))
        self.assertTrue(any("missing skill description" in error for error in errors))

    def test_missing_skill_frontmatter(self):
        self.write("skills/example/SKILL.md", "# Missing metadata\n")
        self.assertTrue(any("missing YAML frontmatter" in error for error in check(self.root)))

    def test_broken_shell_syntax(self):
        self.write("install.sh", "#!/usr/bin/env bash\nif true; then\n")
        self.assertTrue(any("install.sh" in error for error in check(self.root)))

    def test_ignored_files_are_not_checked(self):
        self.write(".gitignore", ".idea/\n")
        self.write(".idea/notes.md", "[Local notes](missing.md)\n")
        self.assertEqual(check(self.root), [])


if __name__ == "__main__":
    unittest.main()
