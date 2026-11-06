package auth

import (
	"errors"
	"strings"
	"sync"
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
	return &memoryRepository{
		users: make([]*User, 0),
	}
}

func (r *memoryRepository) FindByUsername(identifier string) (*User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clean := strings.TrimSpace(identifier)
	for _, u := range r.users {
		if strings.EqualFold(u.Email, clean) || strings.EqualFold(u.Username, clean) {
			return u, nil
		}
	}
	return nil, ErrUserNotFound
}

func (r *memoryRepository) FindByID(id uint) (*User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, ErrUserNotFound
}

func (r *memoryRepository) FindAll() []*User {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*User, len(r.users))
	copy(result, r.users)
	return result
}
