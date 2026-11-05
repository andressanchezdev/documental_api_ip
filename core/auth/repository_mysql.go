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

func (r *mysqlRepository) FindByUsername(identifier string) (*User, error) {
	clean := strings.TrimSpace(identifier)
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
	u.CanRead, u.CanEdit, u.CanStorage = defaultCapabilitiesByRole(u.Role)
	return &u, nil
}

func (r *mysqlRepository) FindByID(id uint) (*User, error) {
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
	u.CanRead, u.CanEdit, u.CanStorage = defaultCapabilitiesByRole(u.Role)
	return &u, nil
}

func (r *mysqlRepository) FindAll() []*User {
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
			u.CanRead, u.CanEdit, u.CanStorage = defaultCapabilitiesByRole(u.Role)
			list = append(list, &u)
		}
	}
	return list
}
