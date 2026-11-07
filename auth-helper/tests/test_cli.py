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

    def test_set_private_permissions_swallows_chmod_errors(self):
        # Windows: os.chmod 가 fd 인자를 거부할 때 (TypeError) 또는
        # 핸들 변환에 실패할 때 (OSError) 호출자가 깨지지 않아야 한다.
        # 권한 설정은 best-effort, 파일 저장은 반드시 진행.
        for exc in (TypeError("fd not supported"), OSError("invalid handle")):
            with self.subTest(exc=type(exc).__name__):
                fake_os = types.SimpleNamespace(chmod=Mock(side_effect=exc))
                with patch.object(cli, "os", fake_os):
                    cli._set_private_permissions(789)  # must not raise


class LoginGuidanceTests(unittest.TestCase):
    def test_qr_state_describes_link_compatible_confirmation(self):
        state = cli.QRState(None)

        with patch.object(cli, "log") as log:
            state.handle_api_payload({"answerLetter": "뻐"})

        log.assert_called_once_with(
            "Answer letter: '뻐' — select this on your phone after opening the login request."
        )


if __name__ == "__main__":
    unittest.main()


class PhoneLoginInputTests(unittest.TestCase):
    """Dummy identity only."""

    def env(self, name="홍길동", birth="900101", phone="010-0000-0000"):
        return {"TOSSCTL_LOGIN_NAME": name, "TOSSCTL_LOGIN_BIRTH": birth, "TOSSCTL_LOGIN_PHONE": phone}

    def test_inputs_come_from_env_and_phone_is_normalized(self):
        with patch.dict(cli.os.environ, self.env(), clear=False):
            self.assertEqual(cli.phone_login_inputs(), ("홍길동", "900101", "01000000000"))

    def test_invalid_inputs_are_rejected_before_any_browser_work(self):
        for bad in (self.env(name=""), self.env(birth="9001"), self.env(phone="02-123-4567")):
            with self.subTest(bad=bad), patch.dict(cli.os.environ, bad, clear=False):
                with self.assertRaises(ValueError):
                    cli.phone_login_inputs()

    def test_phone_flag_does_not_take_personal_data_as_arguments(self):
        args = cli.build_parser().parse_args(["login", "--storage-state", "s.json", "--phone"])
        self.assertTrue(args.phone)
        for attr in ("name", "birth", "phone_number"):
            self.assertFalse(hasattr(args, attr), "personal data must not be argv flags")
