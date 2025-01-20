# S3Backup

## Summary
This program will synchronize one or more of Linux directory structures with Amazon S3 storage.  It is designed to leverage
low cost AWS storage.  Currently this build only supports backing up `rpm` based installations (i.e. Fedora, Redhat, 
CentOS etc.).  Feel free to pull down the source code and compile on another Linux flavor in hopes it may work.

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
