-- ====================================================================
-- SCRIPT DE INICIALIZACIÓN DE BASE DE DATOS Y USUARIOS SEMILLA (RAILWAY)
-- Ecosistema: Gestor Documental (doc-api + doc-premium)
-- ====================================================================

-- 1. Tabla de Usuarios
CREATE TABLE IF NOT EXISTS users (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(190) NULL UNIQUE,
    username VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role ENUM('admin', 'editor', 'reader') NOT NULL DEFAULT 'editor',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 2. Tabla de Metadatos de Documentos
CREATE TABLE IF NOT EXISTS documents (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    drive_id VARCHAR(255) NOT NULL,
    created_by INT UNSIGNED NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_created_by (created_by),
    INDEX idx_drive_id (drive_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 3. Tabla de Permisos de Carpetas (Checklist Granular)
CREATE TABLE IF NOT EXISTS folder_permissions (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    folder_id VARCHAR(255) NOT NULL,
    allowed_user_id INT UNSIGNED NULL,
    allowed_role VARCHAR(50) NULL,
    created_by INT UNSIGNED NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_folder_id (folder_id),
    INDEX idx_allowed_user_id (allowed_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 4. Tabla de Capacidades sobre Google Drive por Usuario
CREATE TABLE IF NOT EXISTS user_drive_permissions (
    user_id INT UNSIGNED PRIMARY KEY,
    can_read TINYINT(1) NOT NULL DEFAULT 1,
    can_edit TINYINT(1) NOT NULL DEFAULT 0,
    can_storage TINYINT(1) NOT NULL DEFAULT 0,
    is_admin TINYINT(1) NOT NULL DEFAULT 0,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ====================================================================
-- INSERTAR / ACTUALIZAR USUARIOS SEMILLA
-- ====================================================================

-- admin@empresa.com  / admin123  (Rol: admin)
-- editor@empresa.com / editor123 (Rol: editor)
-- lector@empresa.com / lector123 (Rol: reader)

INSERT INTO users (id, email, username, password_hash, role) VALUES
(1, 'admin@empresa.com', 'admin', '$2b$10$/baARQ8AjQXiVHGKJPVs2.ghPLwTscHk/SZvd0eUBJI3qpWnHfyT.', 'admin'),
(2, 'editor@empresa.com', 'editor', '$2b$10$9D65f2qVdifWpWw2pIHCf.dJJj3qVUG5KczrFKFwlh38hcsnEdsOS', 'editor'),
(3, 'lector@empresa.com', 'lector', '$2b$10$cog1iYortcuRFWl5AaFRQetw6YbCzCdX96md3VRYN/zyKmvDxeIWy', 'reader')
ON DUPLICATE KEY UPDATE 
    email = VALUES(email),
    password_hash = VALUES(password_hash),
    role = VALUES(role);

-- ====================================================================
-- ASIGNAR PERMISOS Y CAPACIDADES DE GOOGLE DRIVE POR DEFECTO
-- ====================================================================

INSERT INTO user_drive_permissions (user_id, can_read, can_edit, can_storage, is_admin) VALUES
(1, 1, 1, 1, 1), -- Admin: Lectura, Edición, Almacenamiento/Subida, Administración total
(2, 1, 1, 1, 0), -- Editor: Lectura, Edición, Almacenamiento/Subida
(3, 1, 0, 0, 0)  -- Lector: Solo lectura / descarga
ON DUPLICATE KEY UPDATE 
    can_read = VALUES(can_read),
    can_edit = VALUES(can_edit),
    can_storage = VALUES(can_storage),
    is_admin = VALUES(is_admin);

