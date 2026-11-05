package auth

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("correo o contraseña inválidos")
)

func getJWTSecret() []byte {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		secret = "SECRET_KEY_GESTOR_DOC"
	}
	return []byte(secret)
}

type CustomClaims struct {
	UserID uint   `json:"user_id"`
	ID     uint   `json:"id,omitempty"`
	Email  string `json:"email"`
	Role   Role   `json:"role"`
	jwt.RegisteredClaims
}

type Service interface {
	Login(input LoginInput) (*AuthResponse, error)
	ListUsers() []*User
	GetUserByID(id uint) (*User, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) ListUsers() []*User {
	return s.repo.FindAll()
}

func (s *service) GetUserByID(id uint) (*User, error) {
	return s.repo.FindByID(id)
}

func (s *service) Login(input LoginInput) (*AuthResponse, error) {
	identifier := strings.TrimSpace(input.Email)
	if identifier == "" {
		identifier = strings.TrimSpace(input.Username)
	}
	if identifier == "" || input.Password == "" {
		return nil, ErrInvalidCredentials
	}

	user, err := s.repo.FindByUsername(identifier)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		if user.PasswordHash != input.Password {
			return nil, ErrInvalidCredentials
		}
	}

	emailClaim := user.Email
	if emailClaim == "" {
		emailClaim = user.Username
	}

	claims := CustomClaims{
		UserID: user.ID,
		Email:  emailClaim,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(getJWTSecret())
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		Token: signedToken,
		User:  *user,
	}, nil
}
