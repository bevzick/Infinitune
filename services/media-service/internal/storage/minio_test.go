package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
)

type bucketClientStub struct {
	exists bool
	err    error
	bucket string
	key    string
	data   []byte
}

func (stub *bucketClientStub) PutObject(
	_ context.Context,
	bucket string,
	key string,
	reader io.Reader,
	_ int64,
	_ minio.PutObjectOptions,
) (minio.UploadInfo, error) {
	stub.bucket = bucket
	stub.key = key
	stub.data, _ = io.ReadAll(reader)
	return minio.UploadInfo{}, stub.err
}

func (stub *bucketClientStub) BucketExists(_ context.Context, bucket string) (bool, error) {
	stub.bucket = bucket
	return stub.exists, stub.err
}

func (stub *bucketClientStub) GetObject(
	context.Context,
	string,
	string,
	minio.GetObjectOptions,
) (*minio.Object, error) {
	return nil, errors.New("not implemented")
}

func TestCheckRequiresExistingBucket(t *testing.T) {
	tests := []struct {
		name    string
		stub    *bucketClientStub
		wantErr string
	}{
		{name: "bucket exists", stub: &bucketClientStub{exists: true}},
		{name: "bucket missing", stub: &bucketClientStub{}, wantErr: "does not exist"},
		{name: "storage unavailable", stub: &bucketClientStub{err: errors.New("unavailable")}, wantErr: "unavailable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := NewClient(test.stub, "media")
			err := client.Check(context.Background())
			if test.wantErr == "" && err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("Check() error = %v, want containing %q", err, test.wantErr)
			}
			if test.stub.bucket != "media" {
				t.Errorf("BucketExists() bucket = %q", test.stub.bucket)
			}
		})
	}
}

func TestPutStoresObjectInConfiguredBucket(t *testing.T) {
	stub := &bucketClientStub{}
	client := NewClient(stub, "media")
	if err := client.Put(context.Background(), "demo/track.wav",
		bytes.NewBufferString("audio"), 5, "audio/wav"); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if stub.bucket != "media" || stub.key != "demo/track.wav" || string(stub.data) != "audio" {
		t.Fatalf("PutObject() = bucket %q, key %q, data %q", stub.bucket, stub.key, stub.data)
	}
}
