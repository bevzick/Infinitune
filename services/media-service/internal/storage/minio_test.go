package storage

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
)

type bucketClientStub struct {
	exists bool
	err    error
	bucket string
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
