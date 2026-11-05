package routes

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"doc-api/core/auth"
	"doc-api/core/platform/gdrive"
)

type ProductImageResponse struct {
	Success   bool                `json:"success"`
	FileID    string              `json:"file_id"`
	FileName  string              `json:"file_name"`
	MimeType  string              `json:"mime_type"`
	URL       string              `json:"url"`        // URL directa pública de GCP / Drive
	ViewerURL string              `json:"viewer_url"` // URL de Drive
	Producto  *UpdatedProductInfo `json:"producto,omitempty"`
}

type UpdatedProductInfo struct {
	ID          int      `json:"id"`
	Codigo      string   `json:"codigo"`
	ImagenArray []string `json:"imagenArray"`
}

// RegisterImageRoutes registra los endpoints de subida de imágenes y sincronización con la tabla productos
func RegisterImageRoutes(api *echo.Group, client *gdrive.Client, permChecker auth.PermissionChecker, db *sql.DB) {
	// Listar o buscar productos en la tabla productos (con su imagenArray parseado)
	api.GET("/productos", func(c echo.Context) error {
		if db == nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"message": "Base de datos MySQL no disponible"})
		}
		search := strings.TrimSpace(c.QueryParam("q"))
		if search == "" {
			search = strings.TrimSpace(c.QueryParam("codigo"))
		}

		var rows *sql.Rows
		var err error
		if search != "" {
			rows, err = db.Query(`SELECT id, COALESCE(codigo, ''), COALESCE(descripcion, ''), COALESCE(imagenArray, '[]') 
				FROM productos 
				WHERE codigo LIKE ? OR descripcion LIKE ? OR CAST(id AS CHAR) = ? 
				LIMIT 50`, "%"+search+"%", "%"+search+"%", search)
		} else {
			rows, err = db.Query(`SELECT id, COALESCE(codigo, ''), COALESCE(descripcion, ''), COALESCE(imagenArray, '[]') 
				FROM productos 
				ORDER BY id ASC LIMIT 50`)
		}
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		defer rows.Close()

		type ProductoItem struct {
			ID          int      `json:"id"`
			Codigo      string   `json:"codigo"`
			Descripcion string   `json:"descripcion"`
			ImagenArray []string `json:"imagenArray"`
			RawJSON     string   `json:"raw_imagen_array"`
		}

		list := make([]ProductoItem, 0)
		for rows.Next() {
			var p ProductoItem
			if err := rows.Scan(&p.ID, &p.Codigo, &p.Descripcion, &p.RawJSON); err == nil {
				_ = json.Unmarshal([]byte(p.RawJSON), &p.ImagenArray)
				if p.ImagenArray == nil {
					p.ImagenArray = []string{}
				}
				list = append(list, p)
			}
		}
		return c.JSON(http.StatusOK, list)
	}, auth.RequireReadPermission(permChecker))

	// Sincronizar un JSON de ubicaciones de imagen (ej: enviado desde app de imágenes o desde frontend) hacia productos.imagenArray
	api.POST("/productos/imagen-array", func(c echo.Context) error {
		if db == nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"message": "Base de datos MySQL no disponible"})
		}

		var req struct {
			ID          string   `json:"id"`
			ProductoID  string   `json:"producto_id"`
			Codigo      string   `json:"codigo"`
			FolderName  string   `json:"folder_name"`
			Filename    string   `json:"filename"`
			CloudURL    string   `json:"cloud_url"`
			URL         string   `json:"url"`
			ImagenArray []string `json:"imagenArray"`
			Replace     bool     `json:"replace"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "JSON inválido"})
		}

		idLookup := strings.TrimSpace(req.ID)
		if idLookup == "" {
			idLookup = strings.TrimSpace(req.ProductoID)
		}
		codigoLookup := strings.TrimSpace(req.Codigo)
		if codigoLookup == "" && req.FolderName != "" {
			codigoLookup = strings.TrimSpace(req.FolderName)
		}

		targetURL := strings.TrimSpace(req.CloudURL)
		if targetURL == "" {
			targetURL = strings.TrimSpace(req.URL)
		}

		// Si envía el array completo en imagenArray y replace=true
		if req.Replace && len(req.ImagenArray) > 0 {
			prodInfo, err := setProductImageArray(db, idLookup, codigoLookup, req.ImagenArray)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"message": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]interface{}{
				"success":  true,
				"message":  "Campo imagenArray reemplazado correctamente en productos",
				"producto": prodInfo,
			})
		}

		if targetURL == "" && len(req.ImagenArray) > 0 {
			targetURL = req.ImagenArray[0]
		}
		if targetURL == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Se requiere 'url', 'cloud_url' o 'imagenArray'"})
		}

		prodInfo, err := attachImageToProduct(db, idLookup, codigoLookup, targetURL)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": err.Error()})
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":  true,
			"message":  "Ruta de imagen guardada en productos.imagenArray exitosamente",
			"url":      targetURL,
			"producto": prodInfo,
		})
	}, auth.RequireEditPermission(permChecker))

	// Endpoint general y de productos: POST /api/images/upload
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

		// 1. Subir a Google Cloud / Google Drive
		info, err := client.UploadPublicImage(c.Request().Context(), file)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"message": fmt.Sprintf("Error subiendo imagen a GCP: %v", err),
			})
		}

		resp := ProductImageResponse{
			Success:   true,
			FileID:    info.FileID,
			FileName:  info.FileName,
			MimeType:  info.MimeType,
			URL:       info.URL,
			ViewerURL: info.ViewerURL,
		}

		// 2. Si se envía producto_id o codigo, sincronizar con la tabla productos en la base de datos local
		productoIDStr := strings.TrimSpace(c.FormValue("producto_id"))
		if productoIDStr == "" {
			productoIDStr = strings.TrimSpace(c.QueryParam("producto_id"))
		}
		codigoStr := strings.TrimSpace(c.FormValue("codigo"))
		if codigoStr == "" {
			codigoStr = strings.TrimSpace(c.QueryParam("codigo"))
		}

		if (productoIDStr != "" || codigoStr != "") && db != nil {
			prodInfo, updateErr := attachImageToProduct(db, productoIDStr, codigoStr, info.URL)
			if updateErr != nil {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{
					"message": fmt.Sprintf("Imagen subida a GCP con éxito pero error al actualizar BD local: %v", updateErr),
					"gcp_url": info.URL,
				})
			}
			resp.Producto = prodInfo
		}

		return c.JSON(http.StatusCreated, resp)
	}, auth.RequireEditPermission(permChecker))

	// Endpoint dedicado para actualizar productos: POST /api/productos/:id/image
	api.POST("/productos/:id/image", func(c echo.Context) error {
		idParam := strings.TrimSpace(c.Param("id"))
		file, err := c.FormFile("image")
		if err != nil {
			file, err = c.FormFile("file")
		}
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"message": "Archivo de imagen no enviado (campo 'image' o 'file')",
			})
		}

		// 1. Subida a Google Cloud
		info, err := client.UploadPublicImage(c.Request().Context(), file)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"message": fmt.Sprintf("Error subiendo imagen a GCP: %v", err),
			})
		}

		// 2. Actualizar campo imagenArray en la tabla productos
		prodInfo, updateErr := attachImageToProduct(db, idParam, "", info.URL)
		if updateErr != nil {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"message": fmt.Sprintf("Imagen subida a GCP pero falló actualización en MySQL: %v", updateErr),
				"gcp_url": info.URL,
			})
		}

		return c.JSON(http.StatusOK, ProductImageResponse{
			Success:   true,
			FileID:    info.FileID,
			FileName:  info.FileName,
			MimeType:  info.MimeType,
			URL:       info.URL,
			ViewerURL: info.ViewerURL,
			Producto:  prodInfo,
		})
	}, auth.RequireEditPermission(permChecker))
}

func attachImageToProduct(db *sql.DB, idStr, codigoStr, gcpURL string) (*UpdatedProductInfo, error) {
	if db == nil {
		return nil, fmt.Errorf("conexión a base de datos no disponible")
	}

	var id int
	var codigo string
	var rawImageArray sql.NullString

	query := "SELECT id, COALESCE(codigo, ''), imagenArray FROM productos WHERE id = ? OR codigo = ? LIMIT 1"
	err := db.QueryRow(query, idStr, codigoStr).Scan(&id, &codigo, &rawImageArray)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("producto no encontrado con id='%s' o codigo='%s'", idStr, codigoStr)
		}
		return nil, err
	}

	// Parsear el array JSON actual de imagenArray
	var images []string
	if rawImageArray.Valid && strings.TrimSpace(rawImageArray.String) != "" {
		_ = json.Unmarshal([]byte(rawImageArray.String), &images)
	}
	if images == nil {
		images = []string{}
	}

	// Agregar la nueva URL de GCP al arreglo sin duplicarla
	alreadyExists := false
	for _, existing := range images {
		if existing == gcpURL {
			alreadyExists = true
			break
		}
	}
	if !alreadyExists {
		images = append(images, gcpURL)
	}

	// Serializar a formato JSON
	updatedJSONBytes, err := json.Marshal(images)
	if err != nil {
		return nil, fmt.Errorf("error serializando imagenArray: %w", err)
	}

	// Ejecutar UPDATE en la base de datos local
	updateQuery := "UPDATE productos SET imagenArray = ? WHERE id = ?"
	_, err = db.Exec(updateQuery, string(updatedJSONBytes), id)
	if err != nil {
		return nil, fmt.Errorf("error actualizando imagenArray en la tabla productos: %w", err)
	}

	return &UpdatedProductInfo{
		ID:          id,
		Codigo:      codigo,
		ImagenArray: images,
	}, nil
}

func setProductImageArray(db *sql.DB, idStr, codigoStr string, newImages []string) (*UpdatedProductInfo, error) {
	if db == nil {
		return nil, fmt.Errorf("conexión a base de datos no disponible")
	}

	var id int
	var codigo string
	query := "SELECT id, COALESCE(codigo, '') FROM productos WHERE id = ? OR codigo = ? LIMIT 1"
	err := db.QueryRow(query, idStr, codigoStr).Scan(&id, &codigo)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("producto no encontrado con id='%s' o codigo='%s'", idStr, codigoStr)
		}
		return nil, err
	}

	if newImages == nil {
		newImages = []string{}
	}

	updatedJSONBytes, err := json.Marshal(newImages)
	if err != nil {
		return nil, fmt.Errorf("error serializando imagenArray: %w", err)
	}

	if _, err := db.Exec("UPDATE productos SET imagenArray = ? WHERE id = ?", string(updatedJSONBytes), id); err != nil {
		return nil, fmt.Errorf("error actualizando imagenArray: %w", err)
	}

	return &UpdatedProductInfo{
		ID:          id,
		Codigo:      codigo,
		ImagenArray: newImages,
	}, nil
}
