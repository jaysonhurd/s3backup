# S3Backup project instructions

## Project and architecture

- Go 1.27.3; module `github.com/jaysonhurd/s3backup`.
- This CLI backs up local directories to Amazon S3 and explicitly supports
  destructive wipe and sync operations. Preserve confirmation and force flags.
- `cmd` owns startup and flags; `models` owns configuration; `pkg/s3backup`
  uploads files; `pkg/s3clean` removes objects; `pkg/utilities` loads configuration,
  AWS sessions, and logging. `internal` contains logging and telemetry adapters.
- Inject small consumer-owned S3 interfaces. Keep existing public interfaces,
  zerolog constructor arguments, and JSON configuration contracts compatible.
- See [README](README.md) for flags, configuration, build, and verification.

## Development and verification

Use Go 1.27.3+, Python 3.9+, Git, and Bash from the repository root.
No AWS credentials or live S3 service are required for unit tests.

```sh
python3 -m unittest discover -s scripts -p 'test_*.py'
python3 scripts/check_repository.py
bash scripts/validate.sh
```

Format changed Go files using `gofmt`. Run meaningful success and failure tests
with native `testing`; measure coverage and aim for 95% where practical. Explain
coverage gaps instead of adding tests solely to inflate coverage. Tests must use
independent dependencies and temporary directories. Do not run destructive AWS
operations as validation. Generation tooling is tracked through the `tools` tag;
normal builds and tests do not need that tag.

## Go preferences

- Follow Effective Go; prefer simple composition, explicit dependencies, focused
  interfaces, and established package boundaries over speculative abstractions.
- Separate logical sections with blank lines. Group related constants and variable
  declarations at the top of their appropriate scope. Keep mutable state local.
- Document declarations and functions; use JSON tags while retaining wire names.
- Prefer `log/slog` with fixed message constants and structured attributes.
  Zerolog remains supported at public boundaries through an internal adapter.
  Default logs go to `logs/s3backup.log`; keep rotation and explicit paths.
- Define static error values once with `Err` prefixes. Preserve error identity;
  use `errors.Is` and `errors.As`. Do not turn ordinary status messages into errors.
- Accept and propagate contexts for new blocking APIs. Retain legacy API adapters.
- Use OpenTelemetry APIs with caller-owned providers. Do not invent exporter
  endpoints or credentials; do not take ownership of a caller's SDK shutdown.
- Prefer `net/http` for HTTP work and injectable clients exposing
  `Do(*http.Request) (*http.Response, error)` in HTTP client tests.
- Prefer `encoding/json/v2` with explicit compatibility options when needed.
- Never commit secrets, generated logs, IDE state, or local `.codex` state.
  Required JSON exception: `fixtures/tests/*.json` (currently no matching files).
- Explain new production dependencies and significant compatibility tradeoffs.

## Repository checks and delivery

Keep local validation and CI consistent. Validate local Markdown links and
anchors, shell syntax, skill metadata when tracked skills exist, Go formatting,
module checksums, build, tests, and vet. The checker performs structural validation;
it does not verify remote links, AWS integration, or skill behavior.
Use read-only CI tokens, stable check names, timeouts, full SHA action pins, and
checkout without persisted credentials. Local updates do not authorize publishing,
remote enforcement changes, commits, or merging. Preserve license choices and
third-party notices. Report changes, checks, skips, and unresolved issues.
