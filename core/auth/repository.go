package auth

import (
	"errors"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var ErrUserNotFound = errors.New("usuario no encontrado")

type Repository interface {
	FindByUsername(identifier string) (*User, error)
	FindByID(id uint) (*User, error)
	FindAll() []*User
}

type memoryRepository struct {
	mu    sync.RWMutex
	users []*User
}

func NewMemoryRepository() Repository {
	adminHash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	editorHash, _ := bcrypt.GenerateFromPassword([]byte("editor123"), bcrypt.DefaultCost)
	readerHash, _ := bcrypt.GenerateFromPassword([]byte("lector123"), bcrypt.DefaultCost)

	return &memoryRepository{
		users: []*User{
			{
				ID:           1,
				Email:        "admin@empresa.com",
				Username:     "admin",
				PasswordHash: string(adminHash),
				Role:         RoleAdmin,
				CanRead:      true,
				CanEdit:      true,
				CanStorage:   true,
				CreatedAt:    time.Now(),
			},
			{
				ID:           2,
				Email:        "editor@empresa.com",
				Username:     "editor",
				PasswordHash: string(editorHash),
				Role:         RoleEditor,
				CanRead:      true,
				CanEdit:      true,
				CanStorage:   true,
				CreatedAt:    time.Now(),
			},
			{
				ID:           3,
				Email:        "lector@empresa.com",
				Username:     "lector",
				PasswordHash: string(readerHash),
				Role:         RoleReader,
				CanRead:      true,
				CanEdit:      false,
				CanStorage:   false,
				CreatedAt:    time.Now(),
			},
		},
	}
}

func (r *memoryRepository) FindByUsername(identifier string) (*User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clean := strings.ToLower(strings.TrimSpace(identifier))
	for _, u := range r.users {
		if strings.ToLower(u.Email) == clean || strings.ToLower(u.Username) == clean {
			cp := *u
			return &cp, nil
		}
	}
	return nil, ErrUserNotFound
}

func (r *memoryRepository) FindByID(id uint) (*User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, u := range r.users {
		if u.ID == id {
			cp := *u
			return &cp, nil
		}
	}
	return nil, ErrUserNotFound
}

func (r *memoryRepository) FindAll() []*User {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*User, 0, len(r.users))
	for _, u := range r.users {
		cp := *u
		list = append(list, &cp)
	}
	return list
}
