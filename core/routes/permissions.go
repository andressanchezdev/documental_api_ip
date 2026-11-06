package routes

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"doc-api/core/auth"
	"doc-api/core/permissions"
	"doc-api/core/platform/gdrive"
)

// FolderChecklistItem representa un elemento del checklist para la UI de React
type FolderChecklistItem struct {
	FolderID   string   `json:"folder_id"`
	FolderName string   `json:"folder_name"`
	Parents    []string `json:"parents"`
	Allowed    bool     `json:"allowed"`
	Reason     string   `json:"reason,omitempty"` // "assigned", "creator", "admin", "default_open"
}

// RegisterPermissionRoutes registra las rutas para la gestión de capacidades y checklist de carpetas
func RegisterPermissionRoutes(api *echo.Group, client *gdrive.Client, permStore permissions.Store, authSvc auth.Service) {
	// 0. Consultar los permisos activos sobre Drive del propio usuario logueado
	api.GET("/permissions/me", func(c echo.Context) error {
		claims := c.Get("user").(*auth.CustomClaims)
		caps := permStore.GetUserCapabilities(claims.UserID, string(claims.Role))
		return c.JSON(http.StatusOK, caps)
	})

	// 1. Listar usuarios de la DB con sus capacidades actuales sobre Drive
	api.GET("/users", func(c echo.Context) error {
		users := authSvc.ListUsers()
		enriched := make([]auth.User, 0, len(users))
		for _, u := range users {
			cp := *u
			caps := permStore.GetUserCapabilities(u.ID, string(u.Role))
			cp.CanRead = caps.CanRead
			cp.CanEdit = caps.CanEdit
			cp.CanStorage = caps.CanStorage
			enriched = append(enriched, cp)
		}
		return c.JSON(http.StatusOK, enriched)
	}, auth.RequireAdminPermission(permStore))

	permGroup := api.Group("/permissions", auth.RequireAdminPermission(permStore))

	// 2. Obtener lista completa de carpetas y sus permisos actuales (Vista de administrador)
	permGroup.GET("/folders", func(c echo.Context) error {
		folders, err := client.ListFolders(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}

		type FolderPermissionItem struct {
			FolderID       string   `json:"folder_id"`
			FolderName     string   `json:"folder_name"`
			CreatedBy      uint     `json:"created_by"`
			AllowedUserIDs []uint   `json:"allowed_user_ids"`
			AllowedRoles   []string `json:"allowed_roles"`
		}

		allPerms := permStore.GetAllPermissions()
		result := make([]FolderPermissionItem, 0, len(folders))

		for _, f := range folders {
			item := FolderPermissionItem{
				FolderID:       f.ID,
				FolderName:     f.Name,
				AllowedUserIDs: []uint{},
				AllowedRoles:   []string{},
			}
			if cfg, ok := allPerms[f.ID]; ok {
				item.CreatedBy = cfg.CreatedBy
				item.AllowedUserIDs = cfg.AllowedUserIDs
				item.AllowedRoles = cfg.AllowedRoles
			}
			result = append(result, item)
		}

		return c.JSON(http.StatusOK, result)
	})

	// 3. Asignar permisos a una carpeta específica
	permGroup.PUT("/folders/:id", func(c echo.Context) error {
		folderID := strings.TrimSpace(c.Param("id"))
		if folderID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "ID de carpeta requerido"})
		}

		var req struct {
			AllowedUserIDs []uint   `json:"allowed_user_ids"`
			AllowedRoles   []string `json:"allowed_roles"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Datos inválidos"})
		}

		claims := c.Get("user").(*auth.CustomClaims)
		if err := permStore.SetFolderPermissions(folderID, req.AllowedUserIDs, req.AllowedRoles, claims.UserID); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}

		return c.JSON(http.StatusOK, map[string]string{"message": "Permisos de la carpeta actualizados"})
	})

	// 4. Obtener las capacidades (Lectura, Edición, Almacenamiento) + CHECKLIST de carpetas de un usuario
	permGroup.GET("/users/:userId/folders", func(c echo.Context) error {
		uid, err := strconv.ParseUint(c.Param("userId"), 10, 32)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "userId inválido"})
		}
		targetUserID := uint(uid)

		targetRole := "reader"
		if targetUser, err := authSvc.GetUserByID(targetUserID); err == nil && targetUser != nil {
			targetRole = string(targetUser.Role)
		}
		caps := permStore.GetUserCapabilities(targetUserID, targetRole)

		folders, err := client.ListFolders(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}

		allPerms := permStore.GetAllPermissions()
		checklist := make([]FolderChecklistItem, 0, len(folders))

		for _, f := range folders {
			allowed := false
			reason := ""

			if caps.IsAdmin || targetRole == "admin" {
				allowed = true
				reason = "admin"
			} else if cfg, ok := allPerms[f.ID]; ok {
				if cfg.CreatedBy != 0 && cfg.CreatedBy == targetUserID {
					allowed = true
					reason = "creator"
				} else {
					for _, u := range cfg.AllowedUserIDs {
						if u == targetUserID {
							allowed = true
							reason = "assigned"
							break
						}
					}
					if !allowed {
						for _, r := range cfg.AllowedRoles {
							if r == targetRole {
								allowed = true
								reason = "role"
								break
							}
						}
					}
				}
			} else {
				allowed = true
				reason = "default_open"
			}

			checklist = append(checklist, FolderChecklistItem{
				FolderID:   f.ID,
				FolderName: f.Name,
				Parents:    f.Parents,
				Allowed:    allowed,
				Reason:     reason,
			})
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"user_id":      targetUserID,
			"capabilities": caps,
			"folders":      checklist,
		})
	})

	// 5. Guardar capacidades (Lectura, Edición, Almacenamiento) + CHECKLIST de carpetas de un usuario
	permGroup.PUT("/users/:userId/folders", func(c echo.Context) error {
		uid, err := strconv.ParseUint(c.Param("userId"), 10, 32)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "userId inválido"})
		}
		targetUserID := uint(uid)

		var req struct {
			FolderIDs  []string `json:"folder_ids"`
			CanRead    *bool    `json:"can_read"`
			CanEdit    *bool    `json:"can_edit"`
			CanStorage *bool    `json:"can_storage"`
			IsAdmin    *bool    `json:"is_admin"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Datos inválidos"})
		}

		targetRole := "reader"
		if targetUser, err := authSvc.GetUserByID(targetUserID); err == nil && targetUser != nil {
			targetRole = string(targetUser.Role)
		}

		currentCaps := permStore.GetUserCapabilities(targetUserID, targetRole)
		if req.CanRead != nil {
			currentCaps.CanRead = *req.CanRead
		}
		if req.CanEdit != nil {
			currentCaps.CanEdit = *req.CanEdit
		}
		if req.CanStorage != nil {
			currentCaps.CanStorage = *req.CanStorage
		}
		if req.IsAdmin != nil {
			currentCaps.IsAdmin = *req.IsAdmin
		}

		if err := permStore.SetUserCapabilities(currentCaps); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"message": fmt.Sprintf("Error guardando capacidades del usuario: %v", err),
			})
		}

		if req.FolderIDs != nil {
			folders, err := client.ListFolders(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{
					"message": fmt.Sprintf("Error listando carpetas: %v", err),
				})
			}

			allKnownIDs := make([]string, 0, len(folders))
			for _, f := range folders {
				allKnownIDs = append(allKnownIDs, f.ID)
			}

			if err := permStore.SetUserFolderAccess(targetUserID, req.FolderIDs, allKnownIDs); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{
					"message": fmt.Sprintf("Error guardando checklist: %v", err),
				})
			}
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"message":      "Permisos de usuario y carpetas actualizados exitosamente",
			"capabilities": currentCaps,
		})
	})
}
