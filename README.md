# S3Backup

## Summary
This program will synchronize one or more of Linux directory structures with Amazon S3 storage.  It is designed to leverage
low cost AWS storage. It is Go-based and can run on common Linux distributions including Ubuntu, Fedora, and Red Hat Enterprise Linux.

## Releases

Pre-built binaries for all major platforms are published automatically to [GitHub Releases](../../releases) whenever a version tag is pushed.

### Supported platforms

| Platform | Architecture | File |
|---|---|---|
| Linux | amd64 | `s3backup-linux-amd64` |
| Linux | arm64 | `s3backup-linux-arm64` |
| macOS | amd64 (Intel) | `s3backup-darwin-amd64` |
| macOS | arm64 (Apple Silicon) | `s3backup-darwin-arm64` |
| Windows | amd64 | `s3backup-windows-amd64.exe` |
| Windows | arm64 | `s3backup-windows-arm64.exe` |

### Download and run (Linux example)

```bash
# Download the binary
curl -LO https://github.com/jaysonhurd/s3backup/releases/latest/download/s3backup-linux-amd64

# (Optional) verify the checksum
curl -LO https://github.com/jaysonhurd/s3backup/releases/latest/download/s3backup-linux-amd64.sha256
sha256sum -c s3backup-linux-amd64.sha256

# Make executable and run
chmod +x s3backup-linux-amd64
./s3backup-linux-amd64 -config ./config/config.json -backup
```

### Creating a new release

Tag a commit and push the tag — the workflow does the rest:

```bash
git tag v1.0.0
git push origin v1.0.0
```

Pre-release tags (`-alpha`, `-beta`, `-rc`) are automatically marked as pre-releases on GitHub.

---

## Linux Compatibility

This project is distro-agnostic at runtime. Build a static Linux binary and use the same executable on Ubuntu and Fedora/RHEL.

### Build (portable Linux binary)

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/s3backup-linux-amd64 ./cmd
```

### Run

```bash
./bin/s3backup-linux-amd64 -config ./config/config.json -backup
```

### Optional: install to a standard path

```bash
sudo install -m 0755 ./bin/s3backup-linux-amd64 /usr/local/bin/s3backup
```

## Setup
First you need to populate the `config/config.json` file with the approprate configurations.  Some of these you will 
need to first create from within your AWS account:

```azure
{
  "AWS": {
    "S3Region": "us-east-2",
    "S3Bucket": "BUCKET_OF_CHOICE",
    "AccessKeyId": "AWS_ACCESS_KEY_ID",
    "SecretAccessKey": "AWSX_SECRET_ACCESS_KEY",
    "BackupDirectories": [
      "/home/user/directory1",
      "/home/user/directory2",
      "/home/user/directory3",
      "/home/user/directory4"
     ],
    "ACL": "private",
    "ContentDisposition": "attachment",
    "ServerSideEncryption": "AES256",
    "StorageClass": "GLACIER"
  },
  "logging": {
    "logfile_location": "/home/user/backups.log",
    "max_backups": 4,
    "max_size": 1,
    "max_age": 1
  }
}
```
Many of the above configs should be relatively straightforward.  A few caveats: 
- `StorageClass`: Storage classes for AWS S3 can be found [here](https://docs.aws.amazon.com/AmazonS3/latest/userguide/storage-class-intro.html).  The default in the config file is 
`GLACIER`.
- `logfile_location`: Make sure to put your logfile in a valid location.  Also be sure to build some sort of logfile
rotator since this release of `S3Backup` does not rotate logs and they may grow to fill the disk.

## Usage

Run the binary with flags to control backup, wipe, and sync behavior.

```bash
./s3backup [flags]
```

### Flags

| Flag | Type | Default | Description |
| --- | --- | --- | --- |
| `-config` | `string` | `/etc/config.json` | Path to the configuration file. |
| `-backup` | `bool` | `false` | Run backup for directories listed in `AWS.BackupDirectories`. |
| `-sync` | `bool` | `false` | Remove S3 objects that do not exist on local disk. |
| `-wipe` | `bool` | `false` | Delete all objects in the configured S3 bucket. |
| `-force` | `bool` | `false` | Skip confirmation prompt when `-wipe` is used. |
| `-help` | `bool` | `false` | Print help/usage details. |
| `-llevel` | `string` | `info` | Log level (`debug`, `info`, `warn`, `error`, `fatal`, `panic`). |
| `-console` | `bool` | `false` | Enable console logging in addition to logfile output. |

### Behavior Notes

- `-wipe` without `-force` prompts for confirmation.
- `-wipe` can be combined with `-backup` to do a clean-slate backup.
- `-sync` is independent and can be used with or without `-backup`.
- If neither `-backup` nor `-sync` is set, the app initializes and exits after setup checks.

### Examples

Backup using a custom config file:

```bash
./s3backup -config ./config/config.json -backup
```

Wipe bucket and then run a fresh backup:

```bash
./s3backup -config ./config/config.json -wipe -force -backup
```

Sync S3 with local filesystem and enable console logs:

```bash
./s3backup -config ./config/config.json -sync -console -llevel info
```

