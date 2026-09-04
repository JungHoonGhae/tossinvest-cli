import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
CI_WORKFLOW = ROOT / ".github" / "workflows" / "ci.yml"


class CIWorkflowTests(unittest.TestCase):
    def test_dependency_audit_tolerates_registry_outages(self):
        source = CI_WORKFLOW.read_text(encoding="utf-8")
        self.assertIn(
            "pnpm audit --audit-level high --ignore-registry-errors",
            source,
            "registry outages must not hide the later dependency-review result",
        )
        self.assertIn('npm_config_fetch_retries: "1"', source)
        self.assertIn('npm_config_fetch_timeout: "10000"', source)


if __name__ == "__main__":
    unittest.main()
