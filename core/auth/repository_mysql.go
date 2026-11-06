package auth

import (
	"database/sql"
	"errors"
	"strings"
)

type mysqlRepository struct {
	db *sql.DB
}

// NewMySQLRepository crea un repositorio de autenticación conectado a MariaDB
func NewMySQLRepository(db *sql.DB) Repository {
	return &mysqlRepository{db: db}
}

func defaultCapabilitiesByRole(role Role) (canRead, canEdit, canStorage bool) {
	switch role {
	case RoleAdmin, RoleEditor:
		return true, true, true
	default:
		return true, false, false
	}
}

func (r *mysqlRepository) enrichCapabilities(u *User) {
	if u == nil {
		return
	}
	u.CanRead, u.CanEdit, u.CanStorage = defaultCapabilitiesByRole(u.Role)
	var canRead, canEdit, canStorage, isAdmin bool
	err := r.db.QueryRow("SELECT can_read, can_edit, can_storage, is_admin FROM user_drive_permissions WHERE user_id = ? LIMIT 1", u.ID).Scan(&canRead, &canEdit, &canStorage, &isAdmin)
	if err == nil {
		u.CanRead = canRead
		u.CanEdit = canEdit
		u.CanStorage = canStorage
		if isAdmin {
			u.Role = RoleAdmin
		}
	}
}

func (r *mysqlRepository) FindByUsername(identifier string) (*User, error) {
	clean := strings.TrimSpace(identifier)

	// 1. Soporte prioritario para la tabla `usuarios` (DB local de Importadora / MySQL)
	var hasUsuarios int
	_ = r.db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'usuarios'").Scan(&hasUsuarios)

	if hasUsuarios > 0 {
		queryUsuarios := `SELECT 
			id_usuario, 
			COALESCE(NULLIF(email, ''), NULLIF(email_empresa, ''), CAST(id_usuario AS CHAR)), 
			COALESCE(NULLIF(email, ''), CAST(id_usuario AS CHAR)), 
			password, 
			COALESCE(perfil, ''), 
			fecha 
		FROM usuarios 
		WHERE (LOWER(email) = LOWER(?) OR LOWER(email_empresa) = LOWER(?) OR CAST(id_usuario AS CHAR) = ?) 
		ORDER BY estado DESC, id_usuario ASC
		LIMIT 1`
		row := r.db.QueryRow(queryUsuarios, clean, clean, clean)

		var u User
		var perfilStr string
		err := row.Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &perfilStr, &u.CreatedAt)
		if err == nil {
			perfilLower := strings.ToLower(perfilStr)
			if strings.Contains(perfilLower, "admin") {
				u.Role = RoleAdmin
			} else if strings.Contains(perfilLower, "editor") {
				u.Role = RoleEditor
			} else {
				u.Role = RoleReader
			}
			r.enrichCapabilities(&u)
			return &u, nil
		}
	}

	// 2. Fallback a la tabla `users` estándar
	query := `SELECT id, COALESCE(email, username), username, password_hash, role, created_at 
		FROM users 
		WHERE LOWER(email) = LOWER(?) OR LOWER(username) = LOWER(?) 
		LIMIT 1`
	row := r.db.QueryRow(query, clean, clean)

	var u User
	var roleStr string
	err := row.Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &roleStr, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	u.Role = Role(roleStr)
	r.enrichCapabilities(&u)
	return &u, nil
}

func (r *mysqlRepository) FindByID(id uint) (*User, error) {
	// 1. Soporte para tabla `usuarios`
	var hasUsuarios int
	_ = r.db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'usuarios'").Scan(&hasUsuarios)

	if hasUsuarios > 0 {
		queryUsuarios := `SELECT 
			id_usuario, 
			COALESCE(NULLIF(email, ''), NULLIF(email_empresa, ''), CAST(id_usuario AS CHAR)), 
			COALESCE(NULLIF(email, ''), CAST(id_usuario AS CHAR)), 
			password, 
			COALESCE(perfil, ''), 
			fecha 
		FROM usuarios 
		WHERE id_usuario = ? 
		LIMIT 1`
		row := r.db.QueryRow(queryUsuarios, id)

		var u User
		var perfilStr string
		err := row.Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &perfilStr, &u.CreatedAt)
		if err == nil {
			perfilLower := strings.ToLower(perfilStr)
			if strings.Contains(perfilLower, "admin") {
				u.Role = RoleAdmin
			} else if strings.Contains(perfilLower, "editor") {
				u.Role = RoleEditor
			} else {
				u.Role = RoleReader
			}
			r.enrichCapabilities(&u)
			return &u, nil
		}
	}

	// 2. Fallback a tabla `users`
	query := `SELECT id, COALESCE(email, username), username, password_hash, role, created_at 
		FROM users 
		WHERE id = ? 
		LIMIT 1`
	row := r.db.QueryRow(query, id)

	var u User
	var roleStr string
	err := row.Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &roleStr, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	u.Role = Role(roleStr)
	r.enrichCapabilities(&u)
	return &u, nil
}

func (r *mysqlRepository) FindAll() []*User {
	var hasUsuarios int
	_ = r.db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'usuarios'").Scan(&hasUsuarios)

	if hasUsuarios > 0 {
		query := `SELECT id_usuario, COALESCE(NULLIF(email, ''), NULLIF(email_empresa, ''), CAST(id_usuario AS CHAR)), COALESCE(perfil, 'reader'), fecha 
			FROM usuarios ORDER BY id_usuario ASC LIMIT 200`
		rows, err := r.db.Query(query)
		if err == nil {
			defer rows.Close()
			var list []*User
			for rows.Next() {
				var u User
				var perfilStr string
				if err := rows.Scan(&u.ID, &u.Email, &perfilStr, &u.CreatedAt); err == nil {
					u.Username = u.Email
					perfilLower := strings.ToLower(perfilStr)
					if strings.Contains(perfilLower, "admin") {
						u.Role = RoleAdmin
					} else if strings.Contains(perfilLower, "editor") {
						u.Role = RoleEditor
					} else {
						u.Role = RoleReader
					}
					r.enrichCapabilities(&u)
					list = append(list, &u)
				}
			}
			if len(list) > 0 {
				return list
			}
		}
	}

	query := "SELECT id, COALESCE(email, username), username, role, created_at FROM users ORDER BY id ASC"
	rows, err := r.db.Query(query)
	if err != nil {
		return []*User{}
	}
	defer rows.Close()

	var list []*User
	for rows.Next() {
		var u User
		var roleStr string
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &roleStr, &u.CreatedAt); err == nil {
			u.Role = Role(roleStr)
			r.enrichCapabilities(&u)
			list = append(list, &u)
		}
	}
	return list
}
