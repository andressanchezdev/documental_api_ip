package permissions

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// FolderPermissionConfig almacena qué usuarios o roles tienen permitido ver una carpeta
type FolderPermissionConfig struct {
	FolderID       string   `json:"folder_id"`
	AllowedUserIDs []uint   `json:"allowed_user_ids"` // IDs específicos de usuarios con acceso
	AllowedRoles   []string `json:"allowed_roles"`    // Roles con acceso (ej: "editor")
	CreatedBy      uint     `json:"created_by"`       // Creador de la carpeta
}

// UserCapabilities define los permisos granulares sobre Google Drive para un usuario de la DB
type UserCapabilities struct {
	UserID     uint `json:"user_id"`
	CanRead    bool `json:"can_read"`
	CanEdit    bool `json:"can_edit"`
	CanStorage bool `json:"can_storage"`
	IsAdmin    bool `json:"is_admin"`
}

// DefaultCapabilitiesForRole devuelve las capacidades por defecto según el rol base del usuario
func DefaultCapabilitiesForRole(userID uint, role string) UserCapabilities {
	switch role {
	case "admin":
		return UserCapabilities{
			UserID:     userID,
			CanRead:    true,
			CanEdit:    true,
			CanStorage: true,
			IsAdmin:    true,
		}
	case "editor":
		return UserCapabilities{
			UserID:     userID,
			CanRead:    true,
			CanEdit:    true,
			CanStorage: true,
			IsAdmin:    false,
		}
	default:
		return UserCapabilities{
			UserID:     userID,
			CanRead:    true,
			CanEdit:    false,
			CanStorage: false,
			IsAdmin:    false,
		}
	}
}

// Store define los métodos que cualquier almacén de permisos debe implementar
type Store interface {
	CanAccess(folderID string, userID uint, role string) bool
	SetFolderPermissions(folderID string, allowedUserIDs []uint, allowedRoles []string, createdBy uint) error
	SetUserFolderAccess(userID uint, allowedFolderIDs []string, allKnownFolderIDs []string) error
	GetFolderPermissions(folderID string) *FolderPermissionConfig
	GetAllPermissions() map[string]FolderPermissionConfig

	// Capacidades granulares por usuario (Lectura, Edición, Almacenamiento y Admin)
	GetUserCapabilities(userID uint, defaultRole string) UserCapabilities
	SetUserCapabilities(caps UserCapabilities) error
	CanUserRead(userID uint, role string) bool
	CanUserEdit(userID uint, role string) bool
	CanUserStorage(userID uint, role string) bool
	IsUserAdmin(userID uint, role string) bool
}

type fileStore struct {
	mu           sync.RWMutex
	filePath     string
	capsFilePath string
	data         map[string]*FolderPermissionConfig
	userCaps     map[uint]UserCapabilities
}

func NewStore(filePath string) (Store, error) {
	return NewFileStore(filePath)
}

func NewFileStore(filePath string) (Store, error) {
	if filePath == "" {
		filePath = "folder_permissions.json"
	}

	s := &fileStore{
		filePath:     filePath,
		capsFilePath: filePath + ".users.json",
		data:         make(map[string]*FolderPermissionConfig),
		userCaps:     make(map[uint]UserCapabilities),
	}

	if err := s.load(); err != nil {
		if os.IsNotExist(err) {
			_ = s.save()
		} else {
			return nil, fmt.Errorf("error cargando permisos de carpetas: %w", err)
		}
	}

	_ = s.loadCaps()

	return s, nil
}

func (s *fileStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	if len(b) == 0 {
		s.data = make(map[string]*FolderPermissionConfig)
		return nil
	}

	return json.Unmarshal(b, &s.data)
}

func (s *fileStore) save() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, b, 0644)
}

func (s *fileStore) loadCaps() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := os.ReadFile(s.capsFilePath)
	if err != nil {
		return err
	}
	if len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, &s.userCaps)
}

func (s *fileStore) saveCaps() error {
	b, err := json.MarshalIndent(s.userCaps, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.capsFilePath, b, 0644)
}

func (s *fileStore) GetUserCapabilities(userID uint, defaultRole string) UserCapabilities {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if caps, exists := s.userCaps[userID]; exists {
		if defaultRole == "admin" {
			caps.IsAdmin = true
		}
		return caps
	}
	return DefaultCapabilitiesForRole(userID, defaultRole)
}

func (s *fileStore) SetUserCapabilities(caps UserCapabilities) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.userCaps[caps.UserID] = caps
	return s.saveCaps()
}

func (s *fileStore) CanUserRead(userID uint, role string) bool {
	if role == "admin" {
		return true
	}
	return s.GetUserCapabilities(userID, role).CanRead
}

func (s *fileStore) CanUserEdit(userID uint, role string) bool {
	if role == "admin" {
		return true
	}
	return s.GetUserCapabilities(userID, role).CanEdit
}

func (s *fileStore) CanUserStorage(userID uint, role string) bool {
	if role == "admin" {
		return true
	}
	return s.GetUserCapabilities(userID, role).CanStorage
}

func (s *fileStore) IsUserAdmin(userID uint, role string) bool {
	if role == "admin" {
		return true
	}
	return s.GetUserCapabilities(userID, role).IsAdmin
}

func (s *fileStore) CanAccess(folderID string, userID uint, role string) bool {
	if s.IsUserAdmin(userID, role) {
		return true
	}
	if !s.CanUserRead(userID, role) {
		return false
	}
	if folderID == "" || folderID == "root" {
		return true
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	config, exists := s.data[folderID]
	if !exists {
		return true
	}

	if config.CreatedBy != 0 && config.CreatedBy == userID {
		return true
	}

	for _, uid := range config.AllowedUserIDs {
		if uid == userID {
			return true
		}
	}

	for _, r := range config.AllowedRoles {
		if r == role {
			return true
		}
	}

	return false
}

func (s *fileStore) SetFolderPermissions(folderID string, allowedUserIDs []uint, allowedRoles []string, createdBy uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if allowedUserIDs == nil {
		allowedUserIDs = []uint{}
	}
	if allowedRoles == nil {
		allowedRoles = []string{}
	}

	existing, exists := s.data[folderID]
	if exists && createdBy == 0 {
		createdBy = existing.CreatedBy
	}

	s.data[folderID] = &FolderPermissionConfig{
		FolderID:       folderID,
		AllowedUserIDs: allowedUserIDs,
		AllowedRoles:   allowedRoles,
		CreatedBy:      createdBy,
	}

	return s.save()
}

func (s *fileStore) SetUserFolderAccess(userID uint, allowedFolderIDs []string, allKnownFolderIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	allowedSet := make(map[string]bool)
	for _, id := range allowedFolderIDs {
		allowedSet[id] = true
	}

	for _, folderID := range allKnownFolderIDs {
		cfg, exists := s.data[folderID]
		if !exists {
			cfg = &FolderPermissionConfig{
				FolderID:       folderID,
				AllowedUserIDs: []uint{},
				AllowedRoles:   []string{},
			}
			s.data[folderID] = cfg
		}

		shouldHave := allowedSet[folderID]
		has := false
		newUIDs := make([]uint, 0, len(cfg.AllowedUserIDs))

		for _, uid := range cfg.AllowedUserIDs {
			if uid == userID {
				has = true
				if shouldHave {
					newUIDs = append(newUIDs, uid)
				}
			} else {
				newUIDs = append(newUIDs, uid)
			}
		}

		if shouldHave && !has {
			newUIDs = append(newUIDs, userID)
		}

		cfg.AllowedUserIDs = newUIDs
	}

	return s.save()
}

func (s *fileStore) GetFolderPermissions(folderID string) *FolderPermissionConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if cfg, exists := s.data[folderID]; exists {
		return cfg
	}
	return nil
}

func (s *fileStore) GetAllPermissions() map[string]FolderPermissionConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make(map[string]FolderPermissionConfig, len(s.data))
	for k, v := range s.data {
		res[k] = *v
	}
	return res
}
