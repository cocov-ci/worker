package storage

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"
	"os"
	"strings"
)

func NewS3(bucketName string) (Base, error) {
	var configs []func(*config.LoadOptions) error

	// Used by the test suite
	if val, ok := os.LookupEnv("COCOV_WORKER_S3_ENDPOINT"); ok {
		configs = append(configs, config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               val,
				HostnameImmutable: true,
				PartitionID:       "aws",
			}, nil
		})))
	}
	cfg, err := config.LoadDefaultConfig(context.Background(), configs...)
	if err != nil {
		return nil, err
	}
	s3Client := s3.NewFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	_, err = s3Client.HeadBucket(context.Background(), &s3.HeadBucketInput{
		Bucket: &bucketName,
	})
	if err != nil {
		return nil, err
	}

	l := S3Storage{
		log:        zap.L().With(zap.String("facility", "s3-storage")),
		bucketName: aws.String(bucketName),
		client:     s3Client,
		config:     cfg,
	}
	l.log.Info("Initialized S3 Storage adapter", zap.String("bucket_name", bucketName))
	return l, nil
}

type S3Storage struct {
	log        *zap.Logger
	bucketName *string
	client     *s3.Client
	config     aws.Config
}

func (s S3Storage) RepositoryPath(repository string) string {
	return shasum(repository)
}

func (s S3Storage) CommitPath(repository, commitish string) string {
	return strings.Join([]string{s.RepositoryPath(repository), commitish}, "/")
}

func (s S3Storage) download(key string, into *os.File) error {
	downloader := manager.NewDownloader(s.client)
	_, err := downloader.Download(context.Background(), into, &s3.GetObjectInput{
		Bucket: s.bucketName,
		Key:    aws.String(key),
	})

	return err
}

func (s S3Storage) DownloadCommit(repository, commitish, into string) error {
	path := s.CommitPath(repository, commitish)
	brotliPath := path + ".tar.br"
	shasumPath := brotliPath + ".shasum"

	s.log.Info("Downloading compressed repository image from S3",
		zap.String("key", brotliPath),
		zap.Stringp("bucket", s.bucketName))

	outBrotli, err := os.CreateTemp("", "img.*.tar.br")
	if err != nil {
		return err
	}

	defer func(name string) { _ = os.Remove(name) }(outBrotli.Name())

	outSha, err := os.CreateTemp("", "img.*.tar.br.shasum")
	if err != nil {
		return err
	}

	defer func(name string) { _ = os.Remove(name) }(outSha.Name())

	if err = s.download(brotliPath, outBrotli); err != nil {
		return err
	}

	if err = outBrotli.Close(); err != nil {
		return err
	}

	if err = s.download(shasumPath, outSha); err != nil {
		return err
	}

	if err = outSha.Close(); err != nil {
		return err
	}

	if err := validateSha(outSha.Name(), outBrotli.Name()); err != nil {
		return err
	}

	return inflateBrotli(outBrotli.Name(), func(tarPath string) error {
		return untar(tarPath, into)
	})
}
