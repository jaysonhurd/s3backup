#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
python3 -m unittest discover -s scripts -p 'test_*.py'
python3 scripts/check_repository.py
mapfile -d '' files < <(git ls-files --cached --others --exclude-standard -z '*.go')
unformatted=$(gofmt -l "${files[@]}")
if [[ -n "$unformatted" ]]; then
  echo "Unformatted Go files:" >&2
  echo "$unformatted" >&2
  exit 1
fi
go mod verify
go build ./...
coverage=$(mktemp)
trap 'rm -f "$coverage"' EXIT
go test -race -coverprofile="$coverage" ./...
go tool cover -func="$coverage"
go vet ./...
