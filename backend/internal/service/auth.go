package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"github.com/rafay0704/ai-chief-of-staff/backend/internal/auth"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/domain"
	"github.com/rafay0704/ai-chief-of-staff/backend/internal/repository"
)

// AuthService handles registration, login, and user lookup.
type AuthService struct {
	repo   repository.Querier
	tokens *auth.TokenManager
}

// NewAuthService constructs an AuthService.
func NewAuthService(repo repository.Querier, tokens *auth.TokenManager) *AuthService {
	return &AuthService{repo: repo, tokens: tokens}
}

// Register creates a user and returns the user plus a freshly issued token.
func (s *AuthService) Register(ctx context.Context, name, email, password string) (domain.User, string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, "", fmt.Errorf("hash password: %w", err)
	}

	row, err := s.repo.CreateUser(ctx, repository.CreateUserParams{
		Name:         name,
		Email:        strings.ToLower(strings.TrimSpace(email)),
		PasswordHash: string(hash),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.User{}, "", fmt.Errorf("%w: email already registered", domain.ErrConflict)
		}
		return domain.User{}, "", fmt.Errorf("create user: %w", err)
	}

	user := toDomainUser(row)
	token, err := s.tokens.Issue(user.ID)
	if err != nil {
		return domain.User{}, "", err
	}
	return user, token, nil
}

// Login verifies credentials and returns the user plus a token.
// Invalid email and wrong password are intentionally indistinguishable.
func (s *AuthService) Login(ctx context.Context, email, password string) (domain.User, string, error) {
	row, err := s.repo.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, "", domain.ErrUnauthorized
		}
		return domain.User{}, "", fmt.Errorf("get user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(row.PasswordHash), []byte(password)); err != nil {
		return domain.User{}, "", domain.ErrUnauthorized
	}

	user := toDomainUser(row)
	token, err := s.tokens.Issue(user.ID)
	if err != nil {
		return domain.User{}, "", err
	}
	return user, token, nil
}

// GetUser returns the user with the given id.
func (s *AuthService) GetUser(ctx context.Context, id uuid.UUID) (domain.User, error) {
	row, err := s.repo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("get user: %w", err)
	}
	return toDomainUser(row), nil
}

// isUniqueViolation reports whether err is a Postgres unique-constraint error.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
