package document

import (
	"database/sql"
	"errors"
	"time"
)

type mysqlRepository struct {
	db *sql.DB
}

// NewMySQLRepository inicializa el repositorio de documentos en MariaDB
func NewMySQLRepository(db *sql.DB) Repository {
	return &mysqlRepository{db: db}
}

func (r *mysqlRepository) Create(title, driveID string, createdBy uint) (*Document, error) {
	query := "INSERT INTO documents (title, drive_id, created_by) VALUES (?, ?, ?)"
	res, err := r.db.Exec(query, title, driveID, createdBy)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &Document{
		ID:        uint(id),
		Title:     title,
		DriveID:   driveID,
		CreatedBy: createdBy,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (r *mysqlRepository) FindAll() ([]*Document, error) {
	query := "SELECT id, title, drive_id, created_by, created_at, updated_at FROM documents ORDER BY updated_at DESC"
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []*Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.Title, &d.DriveID, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		docs = append(docs, &d)
	}

	return docs, nil
}

func (r *mysqlRepository) FindByID(id uint) (*Document, error) {
	query := "SELECT id, title, drive_id, created_by, created_at, updated_at FROM documents WHERE id = ? LIMIT 1"
	row := r.db.QueryRow(query, id)

	var d Document
	err := row.Scan(&d.ID, &d.Title, &d.DriveID, &d.CreatedBy, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &d, nil
}

func (r *mysqlRepository) Touch(id uint) error {
	query := "UPDATE documents SET updated_at = NOW() WHERE id = ?"
	res, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

