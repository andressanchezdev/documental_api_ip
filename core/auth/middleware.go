package auth

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// PermissionChecker permite al middleware consultar permisos dinámicos en DB (Lectura, Edición, Almacenamiento)
type PermissionChecker interface {
	CanUserRead(userID uint, role string) bool
	CanUserEdit(userID uint, role string) bool
	CanUserStorage(userID uint, role string) bool
	IsUserAdmin(userID uint, role string) bool
}

func JWTMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				return echo.NewHTTPError(http.StatusUnauthorized, "Token requerido")
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			token, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{}, func(t *jwt.Token) (interface{}, error) {
				return getJWTSecret(), nil
			})

			if err != nil || !token.Valid {
				return echo.NewHTTPError(http.StatusUnauthorized, "Token inválido o expirado")
			}

			claims, ok := token.Claims.(*CustomClaims)
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, "Claims inválidos")
			}

			// Soportar tokens externos que envíen 'id' en lugar de 'user_id'
			if claims.UserID == 0 && claims.ID != 0 {
				claims.UserID = claims.ID
			}
			if claims.Role == "" {
				claims.Role = RoleEditor
			}

			c.Set("user", claims)
			return next(c)
		}
	}
}

// RequireAdminPermission verifica que el usuario sea Administrador (por rol o en user_drive_permissions)
func RequireAdminPermission(checker PermissionChecker) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims, ok := c.Get("user").(*CustomClaims)
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, "No autorizado")
			}
			if checker != nil && checker.IsUserAdmin(claims.UserID, string(claims.Role)) {
				return next(c)
			}
			if claims.Role == RoleAdmin {
				return next(c)
			}
			return echo.NewHTTPError(http.StatusForbidden, "Se requieren permisos de administrador")
		}
	}
}

func RequireRole(allowedRoles ...Role) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims, ok := c.Get("user").(*CustomClaims)
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, "No autorizado")
			}

			for _, r := range allowedRoles {
				if claims.Role == r {
					return next(c)
				}
			}

			return echo.NewHTTPError(http.StatusForbidden, "No tienes permisos para realizar esta acción")
		}
	}
}

// RequireReadPermission verifica que el usuario tenga permiso de Lectura (can_read) en la DB
func RequireReadPermission(checker PermissionChecker) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims, ok := c.Get("user").(*CustomClaims)
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, "No autorizado")
			}
			if checker != nil && !checker.CanUserRead(claims.UserID, string(claims.Role)) {
				return echo.NewHTTPError(http.StatusForbidden, "No tienes permiso de lectura sobre Google Drive")
			}
			return next(c)
		}
	}
}

// RequireEditPermission verifica que el usuario tenga permiso de Edición (can_edit) en la DB
func RequireEditPermission(checker PermissionChecker) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims, ok := c.Get("user").(*CustomClaims)
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, "No autorizado")
			}
			if checker != nil {
				if !checker.CanUserEdit(claims.UserID, string(claims.Role)) {
					return echo.NewHTTPError(http.StatusForbidden, "No tienes permiso de edición sobre documentos de Google Drive")
				}
				return next(c)
			}
			if claims.Role != RoleAdmin && claims.Role != RoleEditor {
				return echo.NewHTTPError(http.StatusForbidden, "No tienes permiso de edición")
			}
			return next(c)
		}
	}
}

// RequireStoragePermission verifica que el usuario tenga permiso de Almacenamiento (can_storage) en la DB
func RequireStoragePermission(checker PermissionChecker) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims, ok := c.Get("user").(*CustomClaims)
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, "No autorizado")
			}
			if checker != nil {
				if !checker.CanUserStorage(claims.UserID, string(claims.Role)) {
					return echo.NewHTTPError(http.StatusForbidden, "No tienes permiso de almacenamiento (subir archivos / crear carpetas / mover) en Google Drive")
				}
				return next(c)
			}
			if claims.Role != RoleAdmin && claims.Role != RoleEditor {
				return echo.NewHTTPError(http.StatusForbidden, "No tienes permiso de almacenamiento")
			}
			return next(c)
		}
	}
}
