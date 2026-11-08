package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

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
	GetObject(context.Context, string, string, minio.GetObjectOptions) (*minio.Object, error)
}

var ErrObjectNotFound = errors.New("media object not found")

type Object struct {
	Body         io.ReadCloser
	Size         int64
	ContentType  string
	LastModified time.Time
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

func (client *Client) Open(ctx context.Context, objectKey string) (Object, error) {
	object, err := client.client.GetObject(ctx, client.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return Object{}, mapObjectError(objectKey, err)
	}
	info, err := object.Stat()
	if err != nil {
		_ = object.Close()
		return Object{}, mapObjectError(objectKey, err)
	}
	return Object{
		Body: object, Size: info.Size, ContentType: info.ContentType,
		LastModified: info.LastModified,
	}, nil
}

func mapObjectError(objectKey string, err error) error {
	response := minio.ToErrorResponse(err)
	if response.Code == "NoSuchKey" || response.Code == "NoSuchObject" {
		return fmt.Errorf("object %q: %w", objectKey, ErrObjectNotFound)
	}
	return fmt.Errorf("open object %q: %w", objectKey, err)
}
