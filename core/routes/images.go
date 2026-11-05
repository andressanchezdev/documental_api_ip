package routes

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"doc-api/core/auth"
	"doc-api/core/platform/gdrive"
)

// RegisterImageRoutes registra los endpoints de subida y gestión de imágenes
func RegisterImageRoutes(api *echo.Group, client *gdrive.Client, permChecker auth.PermissionChecker) {
	// Subir imagen genérica a Google Drive con acceso público para usarla en Docs o Sheets
	api.POST("/images/upload", func(c echo.Context) error {
		file, err := c.FormFile("image")
		if err != nil {
			file, err = c.FormFile("file")
		}
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Archivo de imagen no enviado (campo 'image' o 'file')",
			})
		}

		info, err := client.UploadPublicImage(c.Request().Context(), file)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"message": fmt.Sprintf("Error subiendo imagen: %v", err),
			})
		}

		return c.JSON(http.StatusCreated, info)
	}, auth.RequireEditPermission(permChecker))
}
