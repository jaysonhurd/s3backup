# Repository review — 2026-09-16

## Toolchain and dependencies

The module now requires Go 1.27.1. All remaining direct and indirect modules in `go.mod` are at their latest stable versions returned by the Go module proxy on the review date. Lumberjack remains at v2.2.1, its latest release. AWS SDK modules have independent version numbers and were upgraded together.

| Direct module | Previous | Updated |
| --- | --- | --- |
| aws-sdk-go-v2 | v1.41.5 | v1.47.0 |
| aws-sdk-go-v2/config | v1.32.13 | v1.33.5 |
| aws-sdk-go-v2/credentials | v1.19.13 | v1.20.5 |
| aws-sdk-go-v2/service/s3 | v1.97.3 | v1.113.1 |
| smithy-go | v1.24.2 | v1.28.1 |
| zerolog | v1.35.0 | v1.35.1 |
| lumberjack.v2 | v2.2.1 | v2.2.1 |

Removed Counterfeiter and its tooling-only transitive dependencies. The checked-in fake was handwritten, and the old generation directive would replace it with an incompatible API. Tests now use explicit, per-test callbacks without package-global mutable state. Removed the unused `AppConfig` mutex wrapper.

## Fixed findings

| Area | Previous issue | Result |
| --- | --- | --- |
| Backup failures | HEAD, stat, and upload failures were logged and swallowed | Errors propagate to a nonzero CLI exit; cleanup does not follow a failed backup |
| Change detection | S3 upload time was compared with local modification time | SHA-256 content comparison detects changed files with old/preserved timestamps |
| Integrity | No application-provided content checksum | Precomputed SHA-256 is sent to S3; local changes during hashing/upload fail the run |
| Paths | Relative HEAD keys differed from absolute PUT keys | Canonical absolute keys shared across operations, with platform separators normalized |
| Concurrency | Package-global file/error variables were shared by backups | File handles/errors are local; independently constructed backups can run concurrently |
| Remote races | Writes/deletes could act on objects changed since inspection | Conditional creates/overwrites and conditional sync deletion |
| Scope | Sync could delete arbitrary objects in a shared bucket | Only configured directory prefixes and canonical local paths are eligible |
| Pagination | Sync stopped at the first 200 objects | All pages are processed; invalid/repeated continuation tokens fail |
| Local safety | Missing roots could look like deleted files; handles leaked from `os.Open` | Preflight roots, filesystem walk, confined `os.Root` inspection, and closed handles |
| Symlinks | Remote/local paths were not confined | Symlink subtrees and noncanonical keys are preserved; traversal outside roots is prevented |
| Wipe | HTTP-success batch responses with object errors were ignored | Every per-object error is reported |
| CLI | Help continued into config loading; successful wipe-only exited as failure | Early informational exits; consistent failure/success status; explicit operation flow |
| CLI safety | Confirmation could disappear into a logfile | Prompt on stderr and require bucket name; add dry-run and cancellation/timeout |
| Configuration | Example was flat JSON with wrong directory field | Correct nested template and strict decoding/validation before operations |
| Credentials | Temporary static credentials lost session tokens | Session token supported; standard SDK credential/region chain preserved |
| Logging | Logger fed into another logger; setup errors ignored | Direct rotation writer, checked log path, instance log level, close on exit |
| Release | Manual release could build default branch instead of requested tag | Resolve the tag to a commit and build/check that commit |
| Release security | User-controlled tags were interpolated into shell code | Validate tags and pass values through quoted environment variables |
| CI | No tests/vulnerability checks gated releases | Reusable CI checks formatting, module consistency, vet, race tests, vulnerabilities |
| Maintenance | Old action versions and unused release action | Current actions pinned to commit SHAs; native GitHub CLI publishes releases |
| Documentation | Stale rotation, flags, and wipe descriptions | Updated operational behavior, migration guidance, and explicit limits |

## AWS documentation reviewed

AWS MCP documentation searches confirmed the operational behaviors behind these changes:

- [Object Ownership](https://docs.aws.amazon.com/AmazonS3/latest/userguide/about-object-ownership.html): bucket-owner-enforced buckets require omitted ACLs or bucket-owner-full-control.
- [Conditional writes](https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes.html): If-None-Match and If-Match guard concurrent object changes.
- [Conditional deletes](https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-deletes.html): ETag preconditions are supported for general-purpose buckets.
- [Upload limits](https://docs.aws.amazon.com/AmazonS3/latest/userguide/upload-objects.html): larger files require multipart uploads.
- [DeleteObjects response](https://docs.aws.amazon.com/AmazonS3/latest/API/API_DeleteObjects.html): HTTP 200 can contain per-object errors.
- [Go SDK S3 examples](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/go_s3_code_examples.html): multipart transfer is a separate capability from PutObject.

The [Go release history](https://go.dev/doc/devel/release) and live module proxy were used to verify toolchain and library releases. No live AWS resources were modified during review or validation.

## Limits and migration

- This is a reviewed set of concrete fixes, not a guarantee that every possible defect has been eliminated.
- Objects without content-hash metadata are uploaded again on their first run. Expect additional requests, versions, and possibly archive-class retention charges. Use dry-run first.
- Single uploads are limited to 5 GiB. Multipart uploads, restore support, persistent manifests, watch mode, and parallel upload scheduling remain unimplemented features.
- Hashing reads every file. This favors reliable detection over timestamp-only speed. The metadata hash assumes other writers do not change content while retaining stale application metadata.
- The source is not snapshotted. Use filesystem/application snapshots for active databases and other consistency-sensitive workloads. Errors stop subsequent actions, but earlier completed operations are not rolled back.
- Existing symlinks and their subtrees are retained during sync. Absent mounts can still appear as empty directories; source availability remains an operator responsibility.
- Wipe deletes current objects across the entire bucket. It does not permanently purge historical versions or delete markers, or bypass Object Lock.
- Windows key separators are now forward slashes. Old Windows backslash keys are not migrated or automatically deleted. Unix absolute key layout is preserved.
- Changing upload options does not rewrite an object whose stored content hash and size already match. Use an explicit migration if changing storage class, encryption, ACLs, or metadata on existing objects.
- Set up bucket policy, lifecycle, versioning, encryption, and observability for your environment separately. Tests use fakes and a local HTTP S3 endpoint; they do not certify real IAM/KMS/Object Lock configurations.

## Validation

Go 1.27.1 was downloaded from go.dev and its archive SHA-256 verified before use. Validation includes unit/regression tests, local HTTP tests through the actual SDK and CLI, race detection, `go vet`, `go mod verify`, latest-module checks, `govulncheck`, workflow linting, and builds for Linux/macOS/Windows on amd64/arm64.
