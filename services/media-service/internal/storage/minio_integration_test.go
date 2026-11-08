//go:build integration

package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestMinIOConnectionAndBucket(t *testing.T) {
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1 to run integration tests")
	}
	secure, err := strconv.ParseBool(envOrDefault("MINIO_USE_SSL", "false"))
	if err != nil {
		t.Fatalf("parse MINIO_USE_SSL: %v", err)
	}
	client, err := Open(Config{
		Endpoint:  envOrDefault("MINIO_ENDPOINT", "127.0.0.1:9000"),
		AccessKey: requiredEnv(t, "MINIO_ROOT_USER"),
		SecretKey: requiredEnv(t, "MINIO_ROOT_PASSWORD"),
		Bucket:    envOrDefault("MINIO_MEDIA_BUCKET", "infinitune-media"),
		Secure:    secure,
	})
	if err != nil {
		t.Fatalf("open MinIO client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = client.Check(ctx); err != nil {
		t.Fatalf("check MinIO bucket: %v", err)
	}
}

func TestMinIOStreamsPrivateMediaObject(t *testing.T) {
	if os.Getenv("INFINITUNE_INTEGRATION_TESTS") != "1" {
		t.Skip("set INFINITUNE_INTEGRATION_TESTS=1 to run integration tests")
	}
	secure, err := strconv.ParseBool(envOrDefault("MINIO_USE_SSL", "false"))
	if err != nil {
		t.Fatalf("parse MINIO_USE_SSL: %v", err)
	}
	config := Config{
		Endpoint:  envOrDefault("MINIO_ENDPOINT", "127.0.0.1:9000"),
		AccessKey: requiredEnv(t, "MINIO_ROOT_USER"), SecretKey: requiredEnv(t, "MINIO_ROOT_PASSWORD"),
		Bucket: envOrDefault("MINIO_MEDIA_BUCKET", "infinitune-media"), Secure: secure,
	}
	rawClient, err := minio.New(config.Endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""), Secure: config.Secure,
	})
	if err != nil {
		t.Fatalf("create MinIO client: %v", err)
	}
	objectKey := "integration/" + uuid.NewString() + ".m4a"
	want := []byte("integration audio bytes")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = rawClient.PutObject(ctx, config.Bucket, objectKey, bytes.NewReader(want), int64(len(want)),
		minio.PutObjectOptions{ContentType: "audio/mp4"})
	if err != nil {
		t.Fatalf("put media object: %v", err)
	}
	t.Cleanup(func() {
		_ = rawClient.RemoveObject(context.Background(), config.Bucket, objectKey, minio.RemoveObjectOptions{})
	})

	client, err := Open(config)
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	object, err := client.Open(ctx, objectKey)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer object.Body.Close()
	got, err := io.ReadAll(object.Body)
	if err != nil {
		t.Fatalf("read media object: %v", err)
	}
	if !bytes.Equal(got, want) || object.Size != int64(len(want)) || object.ContentType != "audio/mp4" {
		t.Fatalf("object = size %d, content-type %q, body %q", object.Size, object.ContentType, got)
	}
}

func requiredEnv(t *testing.T, key string) string {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("%s is required", key)
	}
	return value
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
