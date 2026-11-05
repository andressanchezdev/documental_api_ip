package auth

import "time"

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleReader Role = "reader"
)

type User struct {
	ID           uint      `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         Role      `json:"role"`
	CanRead      bool      `json:"can_read"`
	CanEdit      bool      `json:"can_edit"`
	CanStorage   bool      `json:"can_storage"`
	CreatedAt    time.Time `json:"created_at"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
