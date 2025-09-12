package auth

import (
	"context"
)

type User struct {
	ID       int
	Username string
	IsGuest  bool
}

type AuthProvider interface {
	// Returns the user ID and error (e.g., if username is taken).
	Register(ctx context.Context, username, password string) (User, error)
	// Login verifies the username and password, returning the user ID if valid.
	Login(ctx context.Context, username, password string) (User, error)
	// GetUser retrieves a User by ID.
	GetUser(ctx context.Context, userID int) (User, error)
	ValidateUser(ctx context.Context, userID int) (bool, error)
}
