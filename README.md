# s3backup

Back up regular files to an Amazon S3 general-purpose bucket. Optionally remove remote files that are missing locally, or wipe all current objects in the bucket.

## Build and test

Requires **Go 1.27.1 or later**.

```sh
go build -o s3backup ./cmd
go test -race ./...
go vet ./...
```

Release builds include Linux, macOS, and Windows binaries for amd64 and arm64. Use `s3backup -version` to check the build tag. Download binaries and matching `.sha256` files from this repository's Releases page; verify with `sha256sum -c <binary>.sha256` on Linux.

## Configuration

Copy [config/config.json.tmpl](config/config.json.tmpl), then set your bucket, region, source directories, and log directory. The configuration must contain nested `AWS` and `Logging` objects. Unknown fields, incomplete credentials, and invalid options produce an error before S3 operations begin.

Use the standard AWS credential chain: environment variables, shared AWS profiles/SSO, or workload IAM roles. `AWS_PROFILE` selects a profile. Region can come from `AWS.S3Region` or the SDK's environment/shared configuration. Credentials are not needed for `-help` or `-version`.

Static `AWS.AccessKeyId` and `AWS.SecretAccessKey` remain supported together. Temporary static credentials also require `AWS.SessionToken`. Prefer temporary credentials; do not commit secrets.

Optional upload settings:

- `ACL`: omitted by default, compatible with buckets that disable ACLs. Explicit `private` ACLs fail on bucket-owner-enforced buckets; use no ACL or `bucket-owner-full-control` where appropriate.
- `StorageClass`: defaults to S3's standard storage behavior when omitted. The template uses `STANDARD`.
- `ServerSideEncryption`: `AES256`, `aws:kms`, or another supported S3 value. Omission uses bucket encryption defaults. Set `SSEKMSKeyID` with KMS encryption to select a key.
- `ContentDisposition`: optional, for example `attachment`.

`Logging.logfile_location` is a **directory**, not a file. Logs are written to `s3backup.log` there and rotated automatically. `max_size` is in megabytes, `max_age` in days, and `max_backups` is a file count; zero uses Lumberjack's default/unlimited retention semantics. Keep logs outside backup directories. New directories/files use restrictive permissions.

## Usage

```sh
# Preview changes first; dry-run prints planned actions to stderr.
./s3backup -config config.json -backup -sync -dry-run

# Back up, then remove missing files only if the backup succeeds.
./s3backup -config config.json -backup -sync -console

# Limit the overall run time; Ctrl-C also cancels requests.
./s3backup -config config.json -backup -timeout 30m

# Wipe current objects, then back up again.
./s3backup -config config.json -wipe -backup
```

| Flag | Default | Behavior |
| --- | --- | --- |
| `-config` | `/etc/config.json` | Configuration file |
| `-backup` | false | Back up configured directories |
| `-sync` | false | Delete missing local files within configured prefixes |
| `-wipe` | false | Delete every current object in the bucket |
| `-force` | false | Skip wipe confirmation; requires `-wipe` |
| `-dry-run` | false | List planned actions without uploading/deleting |
| `-timeout` | `0` | Overall timeout, such as `30m`; zero means no deadline |
| `-console` | false | Also log to stderr |
| `-llevel` | `info` | Logging level |
| `-help` | false | Print usage and exit |
| `-version` | false | Print build version and exit |

No operation flags prints usage. Wipe prompts on stderr and requires typing the bucket name unless `-force` or `-dry-run` is set. Successful wipe-only runs exit successfully. Failures return a nonzero exit code and prevent subsequent operations.

## Backup and cleanup behavior

- Keys are canonical absolute paths with forward slashes, preserving the existing leading slash on Unix: `/home/user/Documents/file.txt`. Relative roots are resolved against the process working directory. Windows keys include the drive prefix. Moving roots or switching platforms does not migrate existing keys.
- Only regular files are uploaded; symlinks, sockets, devices, and empty directories are skipped. Contents are streamed with bounded memory. Files above 5 GiB are rejected with an explicit error; multipart upload is not implemented.
- Files are hashed with SHA-256 on every run. The hash is stored in object metadata and sent to S3 for upload verification. This detects content changes even when timestamps are preserved or moved backward. Source files should be stable; this is not a filesystem snapshot or database backup tool.
- **Migration:** old objects without `s3backup-sha256` metadata are uploaded again on their first backup. This can add request/storage costs and new versions. Preview with `-dry-run` before the first run.
- Conditional writes guard against remote changes between HEAD and PUT. Sync uses listed ETags for conditional deletion. Conflicts fail the run so you can retry after resolving concurrent writers.
- Sync paginates through every configured prefix and preserves unrelated keys, directory markers, noncanonical keys, existing symlinks, and special files. Missing/unreadable roots or filesystem errors stop sync. Removed source roots are not automatically purged from S3. An empty mounted directory still looks empty: verify removable/network filesystems are mounted before syncing.
- Wipe covers the whole bucket, including unrelated prefixes. In versioned buckets, wipe and sync create delete markers; they **do not purge historical versions or existing delete markers**. Object Lock and permission failures are reported, including partial batch failures. A failed run may already have uploaded or deleted some objects; operations are not transactional.
- Dry-run makes read requests and writes local logs, but does not mutate S3. A combined wipe/backup dry-run compares against current objects, so it is not an exact simulation of post-wipe uploads.

## AWS operations

Use a dedicated backup bucket with Block Public Access, encryption at rest, HTTPS, and least-privileged permissions. The application needs `s3:ListBucket` on the bucket and `s3:GetObject`/`s3:PutObject` on its backup prefixes; cleanup additionally needs `s3:DeleteObject`. Explicit ACLs and KMS encryption require their corresponding permissions. Versioning or Object Lock can help protect against accidental deletion; configure retention separately.

Consider S3 access logging, CloudTrail data events, and CloudWatch metrics, with encrypted log destinations. These observability features and KMS usage can add charges. HEAD/PUT/LIST/DELETE requests, retained versions, and archive-class minimum retention/retrieval charges also affect cost. Hashing reads every local file, trading disk I/O for reliable change detection.

See [AWS object ownership guidance](https://docs.aws.amazon.com/AmazonS3/latest/userguide/about-object-ownership.html), [conditional writes](https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes.html), and [S3 pricing](https://aws.amazon.com/s3/pricing/).

The repository review and remaining limits are recorded in [docs/review.md](docs/review.md).
