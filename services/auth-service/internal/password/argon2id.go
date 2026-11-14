package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	defaultMemoryKiB   uint32 = 64 * 1024
	defaultIterations  uint32 = 3
	defaultParallelism uint8  = 4
	defaultSaltLength  uint32 = 16
	defaultKeyLength   uint32 = 32

	minimumMemoryKiB   uint32 = 19 * 1024
	maximumMemoryKiB   uint32 = 256 * 1024
	minimumIterations  uint32 = 2
	maximumIterations  uint32 = 10
	maximumParallelism uint8  = 16
	minimumSaltLength  uint32 = 16
	maximumSaltLength  uint32 = 64
	minimumKeyLength   uint32 = 32
	maximumKeyLength   uint32 = 64
)

var ErrInvalidHash = errors.New("invalid password hash")

type Params struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

func DefaultParams() Params {
	return Params{
		MemoryKiB:   defaultMemoryKiB,
		Iterations:  defaultIterations,
		Parallelism: defaultParallelism,
		SaltLength:  defaultSaltLength,
		KeyLength:   defaultKeyLength,
	}
}

func (p Params) Validate() error {
	if p.MemoryKiB < minimumMemoryKiB || p.MemoryKiB > maximumMemoryKiB {
		return fmt.Errorf("argon2id memory must be between %d and %d KiB", minimumMemoryKiB, maximumMemoryKiB)
	}
	if p.Iterations < minimumIterations || p.Iterations > maximumIterations {
		return fmt.Errorf("argon2id iterations must be between %d and %d", minimumIterations, maximumIterations)
	}
	if p.Parallelism == 0 || p.Parallelism > maximumParallelism {
		return fmt.Errorf("argon2id parallelism must be between 1 and %d", maximumParallelism)
	}
	if p.SaltLength < minimumSaltLength || p.SaltLength > maximumSaltLength {
		return fmt.Errorf("argon2id salt length must be between %d and %d bytes", minimumSaltLength, maximumSaltLength)
	}
	if p.KeyLength < minimumKeyLength || p.KeyLength > maximumKeyLength {
		return fmt.Errorf("argon2id key length must be between %d and %d bytes", minimumKeyLength, maximumKeyLength)
	}

	return nil
}

type Argon2ID struct {
	params Params
	random io.Reader
}

func NewArgon2ID(params Params) (*Argon2ID, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	return &Argon2ID{params: params, random: rand.Reader}, nil
}

func (a *Argon2ID) Hash(plainText string) (string, error) {
	salt := make([]byte, a.params.SaltLength)
	if _, err := io.ReadFull(a.random, salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	key := deriveKey(plainText, salt, a.params)

	return encode(a.params, salt, key), nil
}

func (a *Argon2ID) Verify(plainText, encodedHash string) (bool, error) {
	params, salt, expectedKey, err := decode(encodedHash)
	if err != nil {
		return false, err
	}

	actualKey := deriveKey(plainText, salt, params)

	return subtle.ConstantTimeCompare(actualKey, expectedKey) == 1, nil
}

func deriveKey(plainText string, salt []byte, params Params) []byte {
	return argon2.IDKey(
		[]byte(plainText),
		salt,
		params.Iterations,
		params.MemoryKiB,
		params.Parallelism,
		params.KeyLength,
	)
}

func encode(params Params, salt, key []byte) string {
	encoding := base64.RawStdEncoding

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		params.MemoryKiB,
		params.Iterations,
		params.Parallelism,
		encoding.EncodeToString(salt),
		encoding.EncodeToString(key),
	)
}

func decode(encodedHash string) (Params, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return Params{}, nil, nil, invalidHash("unexpected PHC format")
	}

	version, err := parseUint(parts[2], "v=", 8)
	if err != nil || int(version) != argon2.Version {
		return Params{}, nil, nil, invalidHash("unsupported argon2 version")
	}

	params, err := parseParams(parts[3])
	if err != nil {
		return Params{}, nil, nil, err
	}

	encoding := base64.RawStdEncoding.Strict()
	salt, err := encoding.DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, invalidHash("invalid salt encoding")
	}
	key, err := encoding.DecodeString(parts[5])
	if err != nil {
		return Params{}, nil, nil, invalidHash("invalid key encoding")
	}

	params.SaltLength = uint32(len(salt))
	params.KeyLength = uint32(len(key))
	if err = params.Validate(); err != nil {
		return Params{}, nil, nil, invalidHash(err.Error())
	}

	return params, salt, key, nil
}

func parseParams(encoded string) (Params, error) {
	parts := strings.Split(encoded, ",")
	if len(parts) != 3 {
		return Params{}, invalidHash("unexpected parameter format")
	}

	memory, err := parseUint(parts[0], "m=", 32)
	if err != nil {
		return Params{}, invalidHash("invalid memory parameter")
	}
	iterations, err := parseUint(parts[1], "t=", 32)
	if err != nil {
		return Params{}, invalidHash("invalid iteration parameter")
	}
	parallelism, err := parseUint(parts[2], "p=", 8)
	if err != nil {
		return Params{}, invalidHash("invalid parallelism parameter")
	}

	return Params{
		MemoryKiB:   uint32(memory),
		Iterations:  uint32(iterations),
		Parallelism: uint8(parallelism),
	}, nil
}

func parseUint(value, prefix string, bitSize int) (uint64, error) {
	if !strings.HasPrefix(value, prefix) {
		return 0, ErrInvalidHash
	}

	return strconv.ParseUint(strings.TrimPrefix(value, prefix), 10, bitSize)
}

func invalidHash(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidHash, reason)
}
