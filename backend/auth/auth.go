package auth

import (
	"context"
	"cycling-trainer-agent-ai/backend/models"
	"cycling-trainer-agent-ai/backend/repository"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var jwtKey = []byte("my_secret_key") // In production, use environment variable

// Service defines the business logic for authentication and profiles
type Service interface {
	Register(ctx context.Context, email, password, firstName string) error
	Login(ctx context.Context, email, password string) (string, error)
	GetProfile(ctx context.Context, email string) (*models.User, error)
	UpdateProfile(ctx context.Context, email string, ftp, weight float64) error
}

type authService struct {
	repo repository.Repository
}

// NewService creates a new authentication service
func NewService(repo repository.Repository) Service {
	return &authService{
		repo: repo,
	}
}

func (s *authService) Register(ctx context.Context, email, password, firstName string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	user := &models.User{
		Email:     "USER#" + email,
		SK:        "PROFILE",
		Password:  string(hashedPassword),
		FirstName: firstName,
		FTP:       200,
		Weight:    70,
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return err
	}

	log.Printf("[AUTH] User registered successfully: %s", email)
	return nil
}

func (s *authService) Login(ctx context.Context, email, password string) (string, error) {
	user, err := s.repo.GetUser(ctx, email)
	if err != nil {
		return "", fmt.Errorf("getting user for login: %w", err)
	}
	if user == nil {
		return "", errors.New("user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		log.Printf("[AUTH] Login failed for %s: invalid credentials", email)
		return "", errors.New("invalid credentials")
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &models.Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(jwtKey)
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}

	log.Printf("[AUTH] User logged in successfully: %s", email)
	return signedToken, nil
}

func (s *authService) GetProfile(ctx context.Context, email string) (*models.User, error) {
	return s.repo.GetUser(ctx, email)
}

func (s *authService) UpdateProfile(ctx context.Context, email string, ftp, weight float64) error {
	user, err := s.repo.GetUser(ctx, email)
	if err != nil {
		return fmt.Errorf("getting user for update: %w", err)
	}
	if user == nil {
		return errors.New("user not found")
	}

	user.FTP = ftp
	user.Weight = weight
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		log.Printf("[ERROR] Profile update failed for %s: %v", email, err)
		return err
	}

	// Record History
	timestamp := time.Now().Format(time.RFC3339)
	history := &models.FTPRecord{
		PK:        "USER#" + email,
		SK:        "FTP#" + timestamp,
		FTP:       ftp,
		Weight:    weight,
		Timestamp: timestamp,
	}

	if err := s.repo.SaveFTPRecord(ctx, history); err != nil {
		log.Printf("[WARNING] Profile history record failed for %s: %v", email, err)
		return err
	}

	log.Printf("[PROFILE] Profile and history updated for athlete: %s", email)
	return nil
}
