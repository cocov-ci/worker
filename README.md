# Cocov Worker

Worker contains the implementation of the mechanism responsible for spinning
check containers, executing them against a specific repository commit, and 
pushing information back to the API.

## Usage

TBW

## Testing

In order to execute the full test suite, a local [Redis](https://redis.io) instance running on port
6379 is required alongside a [MinIO](https://min.io/docs/minio/linux/index.html) instance running on 
port 9000.

Redis is required to execute tests in the `redis` package, whilst MinIO is 
required for S3 tests in the `storage` package.

After starting MinIO, make sure to execute `script/bootstrap-minio` in order to
initialize a required bucket and its contents with data required by the S3 spec.
