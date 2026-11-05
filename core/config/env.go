package config

import (
	"bufio"
	"os"
	"strings"
)

type AppConfig struct {
	Port                  string
	AllowedOrigins        []string
	JWTSecret             string
	GoogleCredentialsPath string
	GoogleCredentialsJSON string
	GoogleTokenPath       string
	GoogleTokenJSON       string
	FolderPermissionsPath string
	ExternalAuthURL       string
}

// LoadEnvFile carga pares KEY=VALUE desde un archivo .env si existe (útil en entorno local;
// en Railway las variables ya vienen inyectadas en el entorno del sistema).
func LoadEnvFile(filepath string) {
	file, err := os.Open(filepath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"'`)
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}

// Load carga la configuración general de la aplicación lista para Railway y entorno local.
func Load() AppConfig {
	LoadEnvFile(".env")

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}

	originsRaw := strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS"))
	var allowedOrigins []string
	if originsRaw != "" {
		for _, o := range strings.Split(originsRaw, ",") {
			trimmed := strings.TrimSpace(o)
			if trimmed != "" {
				allowedOrigins = append(allowedOrigins, trimmed)
			}
		}
	} else {
		// Por defecto permite desarrollo local y despliegues en Railway (* si no se restringe)
		allowedOrigins = []string{"*"}
	}

	jwtSecret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if jwtSecret == "" {
		jwtSecret = "SECRET_KEY_GESTOR_DOC"
	}

	credPath := strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if credPath == "" {
		credPath = "credentials.json"
	}

	tokenPath := strings.TrimSpace(os.Getenv("GOOGLE_TOKEN_PATH"))
	if tokenPath == "" {
		tokenPath = "token.json"
	}

	permPath := strings.TrimSpace(os.Getenv("FOLDER_PERMISSIONS_PATH"))
	if permPath == "" {
		permPath = "folder_permissions.json"
	}

	return AppConfig{
		Port:                  port,
		AllowedOrigins:        allowedOrigins,
		JWTSecret:             jwtSecret,
		GoogleCredentialsPath: credPath,
		GoogleCredentialsJSON: strings.TrimSpace(os.Getenv("GOOGLE_CREDENTIALS_JSON")),
		GoogleTokenPath:       tokenPath,
		GoogleTokenJSON:       strings.TrimSpace(os.Getenv("GOOGLE_TOKEN_JSON")),
		FolderPermissionsPath: permPath,
		ExternalAuthURL:       strings.TrimSpace(os.Getenv("EXTERNAL_AUTH_URL")),
	}
}

