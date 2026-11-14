package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/bevzick/Infinitune/services/auth-service/internal/adminbootstrap"
	"github.com/bevzick/Infinitune/services/auth-service/internal/auth"
	"github.com/bevzick/Infinitune/services/auth-service/internal/database"
	postgres_repository "github.com/bevzick/Infinitune/services/auth-service/internal/repository/postgres"
	"github.com/google/uuid"
)

const operationTimeout = 15 * time.Second

func main() {
	email := flag.String("email", os.Getenv("ADMIN_EMAIL"), "registered user email to promote")
	flag.Parse()
	if flag.NArg() != 0 {
		fatalf("unexpected positional arguments")
	}
	config, err := loadDatabaseConfig()
	if err != nil {
		fatalf("configuration: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
	defer cancel()
	pool, err := database.Open(ctx, config)
	if err != nil {
		fatalf("connect to auth database: %v", err)
	}
	defer pool.Close()
	store := &bootstrapStore{
		users: postgres_repository.NewUserRepository(pool),
		roles: postgres_repository.NewRoleAssignmentRepository(pool),
	}
	result, err := adminbootstrap.Promote(ctx, store, *email)
	if err != nil {
		fatalf("promote administrator: %v", err)
	}
	if result.AlreadyAssigned {
		fmt.Printf("Administrator role is already assigned to %s (%s)\n", result.Email, result.UserID)
		return
	}
	fmt.Printf("Assigned administrator role to %s (%s)\n", result.Email, result.UserID)
}

type bootstrapStore struct {
	users *postgres_repository.UserRepository
	roles *postgres_repository.RoleAssignmentRepository
}

func (store *bootstrapStore) FindUserByEmail(ctx context.Context, email string) (auth.User, error) {
	user, err := store.users.ByEmail(ctx, email)
	if errors.Is(err, postgres_repository.ErrNotFound) {
		return auth.User{}, adminbootstrap.ErrUserNotFound
	}
	return user, err
}

func (store *bootstrapStore) AssignAdminRole(ctx context.Context, userID uuid.UUID, now time.Time) error {
	return store.roles.AssignRoleToUser(ctx, userID, auth.RoleAdmin, now)
}

func loadDatabaseConfig() (database.Config, error) {
	portValue := envOrDefault("POSTGRES_PORT", "5432")
	port, err := strconv.ParseUint(portValue, 10, 16)
	if err != nil || port == 0 {
		return database.Config{}, fmt.Errorf("POSTGRES_PORT must be a valid port")
	}
	user, err := requiredEnv("POSTGRES_USER")
	if err != nil {
		return database.Config{}, err
	}
	password, err := requiredEnv("POSTGRES_PASSWORD")
	if err != nil {
		return database.Config{}, err
	}
	name, err := requiredEnv("POSTGRES_AUTH_DB")
	if err != nil {
		return database.Config{}, err
	}
	return database.Config{
		Host: envOrDefault("POSTGRES_HOST", "postgres"), Port: uint16(port),
		User: user, Password: password, Name: name,
	}, nil
}

func requiredEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func fatalf(format string, values ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", values...)
	os.Exit(1)
}
