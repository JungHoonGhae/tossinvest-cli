import sys
import types
import unittest
from pathlib import Path
from unittest.mock import Mock, patch


sys.path.insert(0, str(Path(__file__).resolve().parents[1]))


from tossctl_auth_helper import cli


class PrivatePermissionTests(unittest.TestCase):
    def test_set_private_permissions_uses_chmod_when_fchmod_is_missing(self):
        fake_os = types.SimpleNamespace(chmod=Mock())

        with patch.object(cli, "os", fake_os):
            cli._set_private_permissions(123)

        fake_os.chmod.assert_called_once_with(123, 0o600)

    def test_set_private_permissions_prefers_fchmod_when_available(self):
        fake_os = types.SimpleNamespace(fchmod=Mock(), chmod=Mock())

        with patch.object(cli, "os", fake_os):
            cli._set_private_permissions(456)

        fake_os.fchmod.assert_called_once_with(456, 0o600)
        fake_os.chmod.assert_not_called()


if __name__ == "__main__":
    unittest.main()
