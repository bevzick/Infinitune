package password

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestArgon2IDHashAndVerify(t *testing.T) {
	t.Parallel()

	params := Params{
		MemoryKiB:   minimumMemoryKiB,
		Iterations:  minimumIterations,
		Parallelism: 1,
		SaltLength:  minimumSaltLength,
		KeyLength:   minimumKeyLength,
	}
	hasher, err := NewArgon2ID(params)
	if err != nil {
		t.Fatalf("NewArgon2ID() error = %v", err)
	}
	hasher.random = bytes.NewReader(bytes.Repeat([]byte{0x42}, int(params.SaltLength)))

	encodedHash, err := hasher.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if !strings.HasPrefix(encodedHash, "$argon2id$") {
		t.Fatalf("Hash() = %q", encodedHash)
	}

	verified, err := hasher.Verify("correct horse battery staple", encodedHash)
	if err != nil || !verified {
		t.Fatalf("Verify(correct) = %v, %v", verified, err)
	}
	verified, err = hasher.Verify("wrong password", encodedHash)
	if err != nil || verified {
		t.Fatalf("Verify(wrong) = %v, %v", verified, err)
	}
}

func TestArgon2IDRejectsInvalidHash(t *testing.T) {
	t.Parallel()

	hasher, err := NewArgon2ID(DefaultParams())
	if err != nil {
		t.Fatalf("NewArgon2ID() error = %v", err)
	}

	if _, err = hasher.Verify("password", "not-a-phc-hash"); !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("Verify() error = %v, want ErrInvalidHash", err)
	}
}

func TestArgon2IDParamsValidation(t *testing.T) {
	t.Parallel()

	valid := DefaultParams()
	tests := map[string]func(*Params){
		"memory":      func(params *Params) { params.MemoryKiB = minimumMemoryKiB - 1 },
		"iterations":  func(params *Params) { params.Iterations = minimumIterations - 1 },
		"parallelism": func(params *Params) { params.Parallelism = 0 },
		"salt":        func(params *Params) { params.SaltLength = minimumSaltLength - 1 },
		"key":         func(params *Params) { params.KeyLength = minimumKeyLength - 1 },
	}

	for name, mutate := range tests {
		mutate := mutate
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			params := valid
			mutate(&params)
			if err := params.Validate(); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}
