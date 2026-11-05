package permissions

import (
	"database/sql"
	"fmt"
)

type mysqlStore struct {
	db *sql.DB
}

// NewMySQLStore crea un almacén de permisos de carpetas y capacidades conectado a MariaDB
func NewMySQLStore(db *sql.DB) Store {
	return &mysqlStore{db: db}
}

func (s *mysqlStore) GetUserCapabilities(userID uint, defaultRole string) UserCapabilities {
	if defaultRole == "" {
		_ = s.db.QueryRow("SELECT role FROM users WHERE id = ? LIMIT 1", userID).Scan(&defaultRole)
	}

	var canRead, canEdit, canStorage, isAdmin bool
	query := "SELECT can_read, can_edit, can_storage, is_admin FROM user_drive_permissions WHERE user_id = ? LIMIT 1"
	err := s.db.QueryRow(query, userID).Scan(&canRead, &canEdit, &canStorage, &isAdmin)
	if err != nil {
		return DefaultCapabilitiesForRole(userID, defaultRole)
	}

	if defaultRole == "admin" {
		isAdmin = true
	}

	return UserCapabilities{
		UserID:     userID,
		CanRead:    canRead,
		CanEdit:    canEdit,
		CanStorage: canStorage,
		IsAdmin:    isAdmin,
	}
}

func (s *mysqlStore) SetUserCapabilities(caps UserCapabilities) error {
	query := `INSERT INTO user_drive_permissions (user_id, can_read, can_edit, can_storage, is_admin)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			can_read = VALUES(can_read),
			can_edit = VALUES(can_edit),
			can_storage = VALUES(can_storage),
			is_admin = VALUES(is_admin)`
	_, err := s.db.Exec(query, caps.UserID, caps.CanRead, caps.CanEdit, caps.CanStorage, caps.IsAdmin)
	return err
}

func (s *mysqlStore) CanUserRead(userID uint, role string) bool {
	if role == "admin" {
		return true
	}
	return s.GetUserCapabilities(userID, role).CanRead
}

func (s *mysqlStore) CanUserEdit(userID uint, role string) bool {
	if role == "admin" {
		return true
	}
	return s.GetUserCapabilities(userID, role).CanEdit
}

func (s *mysqlStore) CanUserStorage(userID uint, role string) bool {
	if role == "admin" {
		return true
	}
	return s.GetUserCapabilities(userID, role).CanStorage
}

func (s *mysqlStore) IsUserAdmin(userID uint, role string) bool {
	if role == "admin" {
		return true
	}
	return s.GetUserCapabilities(userID, role).IsAdmin
}

func (s *mysqlStore) CanAccess(folderID string, userID uint, role string) bool {
	if s.IsUserAdmin(userID, role) {
		return true
	}
	if !s.CanUserRead(userID, role) {
		return false
	}
	if folderID == "" || folderID == "root" {
		return true
	}

	var totalRules int
	err := s.db.QueryRow("SELECT COUNT(*) FROM folder_permissions WHERE folder_id = ?", folderID).Scan(&totalRules)
	if err != nil || totalRules == 0 {
		return true
	}

	var matches int
	query := `SELECT COUNT(*) FROM folder_permissions 
		WHERE folder_id = ? AND (created_by = ? OR allowed_user_id = ? OR allowed_role = ?)`
	err = s.db.QueryRow(query, folderID, userID, userID, role).Scan(&matches)
	if err != nil {
		return false
	}

	return matches > 0
}

func (s *mysqlStore) SetFolderPermissions(folderID string, allowedUserIDs []uint, allowedRoles []string, createdBy uint) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if createdBy == 0 {
		_ = tx.QueryRow("SELECT created_by FROM folder_permissions WHERE folder_id = ? AND created_by IS NOT NULL LIMIT 1", folderID).Scan(&createdBy)
	}

	if _, err := tx.Exec("DELETE FROM folder_permissions WHERE folder_id = ?", folderID); err != nil {
		return fmt.Errorf("error limpiando permisos antiguos: %w", err)
	}

	if len(allowedUserIDs) == 0 && len(allowedRoles) == 0 {
		_, err := tx.Exec("INSERT INTO folder_permissions (folder_id, created_by) VALUES (?, ?)", folderID, createdBy)
		if err != nil {
			return err
		}
	}

	for _, uid := range allowedUserIDs {
		_, err := tx.Exec("INSERT INTO folder_permissions (folder_id, allowed_user_id, created_by) VALUES (?, ?, ?)", folderID, uid, createdBy)
		if err != nil {
			return err
		}
	}

	for _, r := range allowedRoles {
		_, err := tx.Exec("INSERT INTO folder_permissions (folder_id, allowed_role, created_by) VALUES (?, ?, ?)", folderID, r, createdBy)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *mysqlStore) SetUserFolderAccess(userID uint, allowedFolderIDs []string, allKnownFolderIDs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM folder_permissions WHERE allowed_user_id = ?", userID); err != nil {
		return fmt.Errorf("error limpiando checklist anterior: %w", err)
	}

	allowedSet := make(map[string]bool, len(allowedFolderIDs))
	for _, id := range allowedFolderIDs {
		allowedSet[id] = true
	}

	// Asegurar que cada carpeta conocida quede registrada (para que las desmarcadas no queden como "sin reglas / default_open")
	for _, folderID := range allKnownFolderIDs {
		var createdBy sql.NullInt64
		_ = tx.QueryRow("SELECT created_by FROM folder_permissions WHERE folder_id = ? AND created_by IS NOT NULL LIMIT 1", folderID).Scan(&createdBy)

		var cbVal interface{}
		if createdBy.Valid {
			cbVal = createdBy.Int64
		}

		if allowedSet[folderID] {
			_, err := tx.Exec("INSERT INTO folder_permissions (folder_id, allowed_user_id, created_by) VALUES (?, ?, ?)", folderID, userID, cbVal)
			if err != nil {
				return err
			}
		} else {
			// Si la carpeta no tiene ningún registro tras borrar a este usuario, dejamos un marcador sin allowed_user_id
			var count int
			_ = tx.QueryRow("SELECT COUNT(*) FROM folder_permissions WHERE folder_id = ?", folderID).Scan(&count)
			if count == 0 {
				_, _ = tx.Exec("INSERT INTO folder_permissions (folder_id, created_by) VALUES (?, ?)", folderID, cbVal)
			}
		}
	}

	return tx.Commit()
}

func (s *mysqlStore) GetFolderPermissions(folderID string) *FolderPermissionConfig {
	rows, err := s.db.Query("SELECT allowed_user_id, allowed_role, created_by FROM folder_permissions WHERE folder_id = ?", folderID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	cfg := &FolderPermissionConfig{
		FolderID:       folderID,
		AllowedUserIDs: []uint{},
		AllowedRoles:   []string{},
	}

	for rows.Next() {
		var uid sql.NullInt64
		var role sql.NullString
		var createdBy sql.NullInt64

		if err := rows.Scan(&uid, &role, &createdBy); err == nil {
			if uid.Valid {
				cfg.AllowedUserIDs = append(cfg.AllowedUserIDs, uint(uid.Int64))
			}
			if role.Valid && role.String != "" {
				cfg.AllowedRoles = append(cfg.AllowedRoles, role.String)
			}
			if createdBy.Valid && cfg.CreatedBy == 0 {
				cfg.CreatedBy = uint(createdBy.Int64)
			}
		}
	}

	return cfg
}

func (s *mysqlStore) GetAllPermissions() map[string]FolderPermissionConfig {
	rows, err := s.db.Query("SELECT folder_id, allowed_user_id, allowed_role, created_by FROM folder_permissions")
	if err != nil {
		return make(map[string]FolderPermissionConfig)
	}
	defer rows.Close()

	res := make(map[string]*FolderPermissionConfig)
	for rows.Next() {
		var folderID string
		var uid sql.NullInt64
		var role sql.NullString
		var createdBy sql.NullInt64

		if err := rows.Scan(&folderID, &uid, &role, &createdBy); err == nil {
			cfg, exists := res[folderID]
			if !exists {
				cfg = &FolderPermissionConfig{
					FolderID:       folderID,
					AllowedUserIDs: []uint{},
					AllowedRoles:   []string{},
				}
				res[folderID] = cfg
			}

			if uid.Valid {
				cfg.AllowedUserIDs = append(cfg.AllowedUserIDs, uint(uid.Int64))
			}
			if role.Valid && role.String != "" {
				cfg.AllowedRoles = append(cfg.AllowedRoles, role.String)
			}
			if createdBy.Valid && cfg.CreatedBy == 0 {
				cfg.CreatedBy = uint(createdBy.Int64)
			}
		}
	}

	finalMap := make(map[string]FolderPermissionConfig, len(res))
	for k, v := range res {
		finalMap[k] = *v
	}
	return finalMap
}
