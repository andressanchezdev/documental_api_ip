package document

import (
	"errors"
	"sync"
	"time"
)

var ErrNotFound = errors.New("documento no encontrado")

type Repository interface {
	Create(title, driveID string, createdBy uint) (*Document, error)
	FindAll() ([]*Document, error)
	FindByID(id uint) (*Document, error)
	Touch(id uint) error
}

type memoryRepository struct {
	mu     sync.RWMutex
	docs   map[uint]*Document
	nextID uint
}

func NewMemoryRepository() Repository {
	return &memoryRepository{
		docs:   make(map[uint]*Document),
		nextID: 1,
	}
}

func (r *memoryRepository) Create(title, driveID string, createdBy uint) (*Document, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	doc := &Document{
		ID:        r.nextID,
		Title:     title,
		DriveID:   driveID,
		CreatedBy: createdBy,
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.docs[r.nextID] = doc
	r.nextID++

	return doc, nil
}

func (r *memoryRepository) FindAll() ([]*Document, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res := make([]*Document, 0, len(r.docs))
	for _, doc := range r.docs {
		res = append(res, doc)
	}
	return res, nil
}

func (r *memoryRepository) FindByID(id uint) (*Document, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	doc, ok := r.docs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return doc, nil
}

func (r *memoryRepository) Touch(id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	doc, ok := r.docs[id]
	if !ok {
		return ErrNotFound
	}
	doc.UpdatedAt = time.Now()
	return nil
}