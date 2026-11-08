//go:build integration

package storage

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"
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
