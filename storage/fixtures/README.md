### Cocov Worker Storage Fixtures

This directory contains files used by specs to assert specific behaviours.
Before making changes to this directory, take note of which files are used by
which specs, and run specs again after changing contents here.

`test.tar.zst`: Used by `TestInflateZstd`, in `storage/base_test.go`.

`shasum_test`: This directory and all of its contents are used by 
`Test_validateSha`, in `storage/base_test.go`.

`fake-s3`: The content of this directory is used by `script/bootstrap-minio` and
`storage/local_test.go`.
