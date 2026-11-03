# Authenticated Sanitized Fixtures

Response shapes for authenticated endpoints. Every value here is synthetic — identifiers are
redacted and amounts, quantities, and product codes are generated.

Refresh them with both steps. `fetch_auth_fixtures.py` only redacts identifier keys
(`accountNo`, `name`, …); `scrub_fixture_values.py` replaces the remaining values so the
committed fixture is dummy data end to end:

```bash
python3 tools/fetch_auth_fixtures.py output/playwright/tossinvest-account-state.json fixtures/responses/auth-sanitized
python3 tools/scrub_fixture_values.py fixtures/responses/auth-sanitized
```

Rules:

- never commit raw storage state
- never commit raw account numbers, cookies, or tokens
- commit synthetic amounts, quantities, and product codes only — run the scrub step
- inspect diffs before commit when fixture values change

> `TOSSCTL_MOCK_DIR` is not implemented: no code reads it, so setting it
> changes nothing and the command above would query the **live account API**.
> Do not run fixture paths against live commands expecting mocks.
