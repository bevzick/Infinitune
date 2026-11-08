package storage

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Secure    bool
}

type bucketClient interface {
	BucketExists(context.Context, string) (bool, error)
}

type Client struct {
	client bucketClient
	bucket string
}

func Open(config Config) (*Client, error) {
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: config.Secure,
	})
	if err != nil {
		return nil, fmt.Errorf("create object storage client: %w", err)
	}
	return NewClient(client, config.Bucket), nil
}

func NewClient(client bucketClient, bucket string) *Client {
	return &Client{client: client, bucket: bucket}
}

func (client *Client) Check(ctx context.Context) error {
	exists, err := client.client.BucketExists(ctx, client.bucket)
	if err != nil {
		return fmt.Errorf("check media bucket %q: %w", client.bucket, err)
	}
	if !exists {
		return fmt.Errorf("media bucket %q does not exist", client.bucket)
	}
	return nil
}
