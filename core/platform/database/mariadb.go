package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

// LoadConfigFromEnv carga la configuración de MariaDB desde variables de entorno
func LoadConfigFromEnv() Config {
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "3306"
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "root"
	}
	password := os.Getenv("DB_PASSWORD")
	database := os.Getenv("DB_NAME")
	if database == "" {
		database = "doc_api"
	}

	return Config{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Database: database,
	}
}

// ConnectMariaDB abre una conexión y pool hacia MariaDB
func ConnectMariaDB(cfg Config) (*sql.DB, error) {
	dsnOverride := os.Getenv("DB_DSN")
	dsn := dsnOverride
	if dsn == "" {
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.User,
			cfg.Password,
			cfg.Host,
			cfg.Port,
			cfg.Database,
		)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("error abriendo conexión con MariaDB: %w", err)
	}

	// Configuración de connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("fallo al hacer ping a MariaDB en %s:%s (BD: %s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}

	log.Printf("[MariaDB] Conexión establecida exitosamente con la base de datos '%s'", cfg.Database)

	if err := AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("error inicializando tablas en MariaDB: %w", err)
	}

	return db, nil
}

// AutoMigrate crea y actualiza las tablas requeridas si aún no existen
func AutoMigrate(db *sql.DB) error {
	queries := []string{
		// 1. Tabla de usuarios (con soporte de email y username)
		`CREATE TABLE IF NOT EXISTS users (
			id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			email VARCHAR(190) NULL UNIQUE,
			username VARCHAR(100) NOT NULL UNIQUE,
			password_hash VARCHAR(255) NOT NULL,
			role ENUM('admin', 'editor', 'reader') NOT NULL DEFAULT 'editor',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		// 2. Tabla de documentos
		`CREATE TABLE IF NOT EXISTS documents (
			id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			drive_id VARCHAR(255) NOT NULL,
			created_by INT UNSIGNED NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_created_by (created_by),
			INDEX idx_drive_id (drive_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		// 3. Tabla de permisos de carpetas (checklist)
		`CREATE TABLE IF NOT EXISTS folder_permissions (
			id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			folder_id VARCHAR(255) NOT NULL,
			allowed_user_id INT UNSIGNED NULL,
			allowed_role VARCHAR(50) NULL,
			created_by INT UNSIGNED NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_folder_id (folder_id),
			INDEX idx_allowed_user_id (allowed_user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,

		// 4. Tabla de capacidades de Drive por usuario (Lectura, Edición, Almacenamiento, Admin)
		`CREATE TABLE IF NOT EXISTS user_drive_permissions (
			user_id INT UNSIGNED PRIMARY KEY,
			can_read TINYINT(1) NOT NULL DEFAULT 1,
			can_edit TINYINT(1) NOT NULL DEFAULT 0,
			can_storage TINYINT(1) NOT NULL DEFAULT 0,
			is_admin TINYINT(1) NOT NULL DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}

	// Asegurar columna email si la tabla users fue creada por una versión previa
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN email VARCHAR(190) NULL UNIQUE AFTER id;")

	// Insertar usuarios iniciales por defecto si la tabla está vacía
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err == nil && count == 0 {
		adminHash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		editorHash, _ := bcrypt.GenerateFromPassword([]byte("editor123"), bcrypt.DefaultCost)
		readerHash, _ := bcrypt.GenerateFromPassword([]byte("lector123"), bcrypt.DefaultCost)

		seedQuery := `INSERT INTO users (email, username, password_hash, role) VALUES 
			('admin@empresa.com', 'admin', ?, 'admin'),
			('editor@empresa.com', 'editor', ?, 'editor'),
			('lector@empresa.com', 'lector', ?, 'reader');`
		_, _ = db.Exec(seedQuery, string(adminHash), string(editorHash), string(readerHash))
		log.Println("[MariaDB] Usuarios semilla creados (admin@empresa.com, editor@empresa.com, lector@empresa.com)")
	} else {
		// Poblar email por defecto en registros antiguos si estuviera vacío
		_, _ = db.Exec("UPDATE users SET email = CONCAT(username, '@empresa.com') WHERE email IS NULL OR email = '';")
	}

	return nil
}
