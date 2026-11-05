package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"doc-api/core/auth"
	"doc-api/core/config"
	"doc-api/core/permissions"
	"doc-api/core/platform/database"
	"doc-api/core/platform/gdrive"
	"doc-api/core/routes"
)

func main() {
	// 0. Cargar variables de entorno (.env local o variables de Railway)
	appCfg := config.Load()

	// 1. Inicializar cliente de Google Drive, Docs y Sheets
	gdriveClient, err := gdrive.NewClient(appCfg.GoogleCredentialsPath)
	if err != nil {
		log.Fatalf("Fallo al inicializar Google Drive: %v", err)
	}

	// 2. Inicializar Conexión a MariaDB
	dbCfg := database.LoadConfigFromEnv()
	db, dbErr := database.ConnectMariaDB(dbCfg)

	var authRepo auth.Repository
	var permStore permissions.Store

	if dbErr != nil {
		log.Printf("[Advertencia] No se pudo conectar a MariaDB (%v). Usando repositorio en memoria y JSON local.", dbErr)
		authRepo = auth.NewMemoryRepository()

		var err error
		permStore, err = permissions.NewStore(appCfg.FolderPermissionsPath)
		if err != nil {
			log.Fatalf("Fallo al inicializar almacén de permisos: %v", err)
		}
	} else {
		defer db.Close()
		authRepo = auth.NewMySQLRepository(db)
		permStore = permissions.NewMySQLStore(db)
	}

	authSvc := auth.NewService(authRepo)

	// 3. Instanciar Servidor Echo con CORS configurable para Railway
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: appCfg.AllowedOrigins,
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderAuthorization, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderOrigin},
	}))

	// Healthcheck para Railway
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Rutas Públicas: Login con email y password (enriquecido con permisos de Drive desde DB)
	e.POST("/api/auth/login", func(c echo.Context) error {
		var input auth.LoginInput
		if err := c.Bind(&input); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Datos de entrada inválidos")
		}

		res, err := authSvc.Login(input)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
		}

		caps := permStore.GetUserCapabilities(res.User.ID, string(res.User.Role))
		res.User.CanRead = caps.CanRead
		res.User.CanEdit = caps.CanEdit
		res.User.CanStorage = caps.CanStorage

		return c.JSON(http.StatusOK, res)
	})

	readGuard := auth.RequireReadPermission(permStore)
	editGuard := auth.RequireEditPermission(permStore)
	storageGuard := auth.RequireStoragePermission(permStore)

	// Rutas Protegidas por JWT
	api := e.Group("/api", auth.JWTMiddleware())
	{
		// 1. Consulta de cuota de almacenamiento de Drive
		api.GET("/storage", func(c echo.Context) error {
			quota, err := gdriveClient.GetStorageQuota(c.Request().Context())
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
			}
			return c.JSON(http.StatusOK, quota)
		}, readGuard)

		// 2. Creación de Carpetas en Drive (Requiere permiso de Almacenamiento can_storage)
		api.POST("/folders", func(c echo.Context) error {
			var req struct {
				Name           string   `json:"name"`
				ParentID       string   `json:"parent_id"`
				AllowedUserIDs []uint   `json:"allowed_user_ids"`
				AllowedRoles   []string `json:"allowed_roles"`
			}
			if err := c.Bind(&req); err != nil || strings.TrimSpace(req.Name) == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"message": "El nombre de la carpeta es requerido"})
			}

			claims := c.Get("user").(*auth.CustomClaims)
			folderID, err := gdriveClient.CreateFolder(c.Request().Context(), strings.TrimSpace(req.Name), req.ParentID)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
			}

			// Registrar permisos de la nueva carpeta en el checklist
			_ = permStore.SetFolderPermissions(folderID, req.AllowedUserIDs, req.AllowedRoles, claims.UserID)

			parents := []string{}
			if strings.TrimSpace(req.ParentID) != "" && req.ParentID != "root" {
				parents = append(parents, strings.TrimSpace(req.ParentID))
			}

			return c.JSON(http.StatusCreated, map[string]interface{}{
				"id":         folderID,
				"name":       strings.TrimSpace(req.Name),
				"title":      strings.TrimSpace(req.Name),
				"parents":    parents,
				"created_by": claims.UserID,
			})
		}, storageGuard)

		// 3. Subgrupo de Documentos y Explorador de Drive (/api/docs)
		docsGroup := api.Group("/docs")
		{
			// Listar documentos desde Drive filtrados por permisos de carpeta del usuario
			docsGroup.GET("", func(c echo.Context) error {
				folderID := strings.TrimSpace(c.QueryParam("folder_id"))
				driveDocs, err := gdriveClient.ListDocsByFolder(c.Request().Context(), folderID)
				if err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
				}

				claims := c.Get("user").(*auth.CustomClaims)

				type DocItem struct {
					ID        string    `json:"id"`
					Title     string    `json:"title"`
					Name      string    `json:"name"`
					MimeType  string    `json:"mime_type"`
					Parents   []string  `json:"parents"`
					UpdatedAt time.Time `json:"updated_at"`
				}

				result := make([]DocItem, 0, len(driveDocs))
				for _, d := range driveDocs {
					// Verificar acceso sobre las carpetas contenedoras del documento
					canSee := true
					for _, pid := range d.Parents {
						if !permStore.CanAccess(pid, claims.UserID, string(claims.Role)) {
							canSee = false
							break
						}
					}
					if !canSee {
						continue
					}

					result = append(result, DocItem{
						ID:        d.ID,
						Title:     d.Name,
						Name:      d.Name,
						MimeType:  d.MimeType,
						Parents:   d.Parents,
						UpdatedAt: d.UpdatedAt,
					})
				}

				return c.JSON(http.StatusOK, result)
			}, readGuard)

			// Vista previa: exportar a PDF directo desde Drive
			docsGroup.GET("/:id/preview", func(c echo.Context) error {
				fileID := strings.TrimSpace(c.Param("id"))
				if fileID == "" {
					return c.JSON(http.StatusBadRequest, map[string]string{"message": "ID inválido"})
				}

				stream, err := gdriveClient.ExportPDFStream(c.Request().Context(), fileID)
				if err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{
						"message": fmt.Sprintf("Error exportando a PDF: %v", err),
					})
				}
				defer stream.Close()

				c.Response().Header().Set(echo.HeaderContentType, "application/pdf")
				c.Response().Header().Set("Content-Disposition", "inline; filename=\"documento.pdf\"")

				_, copyErr := io.Copy(c.Response().Writer, stream)
				return copyErr
			}, readGuard)

			// Listar carpetas filtradas por el Checklist de Permisos
			docsGroup.GET("/folders", func(c echo.Context) error {
				folders, err := gdriveClient.ListFolders(c.Request().Context())
				if err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
				}

				claims := c.Get("user").(*auth.CustomClaims)

				filtered := make([]gdrive.DriveFileMetadata, 0, len(folders))
				for _, f := range folders {
					if permStore.CanAccess(f.ID, claims.UserID, string(claims.Role)) {
						filtered = append(filtered, f)
					}
				}

				return c.JSON(http.StatusOK, filtered)
			}, readGuard)

			// Mover archivo (Drag & Drop) - Requiere permiso de almacenamiento (can_storage)
			docsGroup.PUT("/:id/move", func(c echo.Context) error {
				fileID := strings.TrimSpace(c.Param("id"))
				if fileID == "" {
					return c.JSON(http.StatusBadRequest, map[string]string{"message": "ID de archivo requerido"})
				}

				var req struct {
					TargetFolderID string `json:"target_folder_id"`
				}
				if err := c.Bind(&req); err != nil {
					return c.JSON(http.StatusBadRequest, map[string]string{"message": "Datos inválidos"})
				}

				if err := gdriveClient.MoveFile(c.Request().Context(), fileID, req.TargetFolderID); err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{
						"message": fmt.Sprintf("Error moviendo archivo: %v", err),
					})
				}

				return c.JSON(http.StatusOK, map[string]string{"message": "Archivo movido exitosamente"})
			}, storageGuard)

			// Extraer texto plano del documento
			docsGroup.GET("/:id/content", func(c echo.Context) error {
				fileID := strings.TrimSpace(c.Param("id"))
				if fileID == "" {
					return c.JSON(http.StatusBadRequest, map[string]string{"message": "ID inválido"})
				}

				content, err := gdriveClient.ExtractText(c.Request().Context(), fileID)
				if err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{
						"message": fmt.Sprintf("Error extrayendo texto: %v", err),
					})
				}

				return c.JSON(http.StatusOK, map[string]string{"content": content})
			}, readGuard)

			// Guardar cambios en el texto (Requiere permiso de Edición can_edit)
			docsGroup.PUT("/:id/content", func(c echo.Context) error {
				fileID := strings.TrimSpace(c.Param("id"))
				if fileID == "" {
					return c.JSON(http.StatusBadRequest, map[string]string{"message": "ID inválido"})
				}

				var req struct {
					Content string `json:"content"`
				}
				if err := c.Bind(&req); err != nil {
					return c.JSON(http.StatusBadRequest, map[string]string{"message": "Datos inválidos"})
				}

				if err := gdriveClient.UpdateText(c.Request().Context(), fileID, req.Content); err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{
						"message": fmt.Sprintf("Error guardando cambios: %v", err),
					})
				}

				return c.JSON(http.StatusOK, map[string]string{"message": "Guardado exitosamente"})
			}, editGuard)

			// Obtener matriz de celdas para hojas de cálculo
			docsGroup.GET("/:id/sheet-data", func(c echo.Context) error {
				fileID := strings.TrimSpace(c.Param("id"))
				data, err := gdriveClient.GetSheetData(c.Request().Context(), fileID)
				if err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{
						"message": fmt.Sprintf("Error leyendo hoja: %v", err),
					})
				}
				return c.JSON(http.StatusOK, data)
			}, readGuard)

			// Guardar matriz de celdas modificada (Requiere permiso de Edición can_edit)
			docsGroup.PUT("/:id/sheet-data", func(c echo.Context) error {
				fileID := strings.TrimSpace(c.Param("id"))
				var req struct {
					SheetName string     `json:"sheet_name"`
					Rows      [][]string `json:"rows"`
				}
				if err := c.Bind(&req); err != nil {
					return c.JSON(http.StatusBadRequest, map[string]string{"message": "Datos inválidos"})
				}

				if err := gdriveClient.UpdateSheetData(c.Request().Context(), fileID, req.SheetName, req.Rows); err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{
						"message": fmt.Sprintf("Error guardando hoja: %v", err),
					})
				}

				return c.JSON(http.StatusOK, map[string]string{"message": "Hoja de cálculo actualizada"})
			}, editGuard)

			// Descarga de documentos exportados a PDF, DOCX o XLSX
			docsGroup.GET("/:id/download", func(c echo.Context) error {
				fileID := strings.TrimSpace(c.Param("id"))
				if fileID == "" {
					return c.JSON(http.StatusBadRequest, map[string]string{"message": "ID inválido"})
				}

				format := strings.ToLower(c.QueryParam("format"))
				mimeType := "application/pdf"
				ext := "pdf"

				switch format {
				case "docx", "doc":
					mimeType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
					ext = "docx"
				case "xlsx", "xls":
					mimeType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
					ext = "xlsx"
				}

				stream, err := gdriveClient.ExportDocumentStream(c.Request().Context(), fileID, mimeType)
				if err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{
						"message": fmt.Sprintf("Error al preparar la descarga: %v", err),
					})
				}
				defer stream.Close()

				filename := fmt.Sprintf("documento_%s.%s", fileID, ext)
				c.Response().Header().Set(echo.HeaderContentType, mimeType)
				c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

				_, copyErr := io.Copy(c.Response().Writer, stream)
				return copyErr
			}, readGuard)

			// Subir documento y convertirlo a Google Doc o Sheet dentro de la carpeta indicada (can_storage)
			docsGroup.POST("/upload", func(c echo.Context) error {
				file, err := c.FormFile("file")
				if err != nil {
					return c.JSON(http.StatusBadRequest, map[string]string{"message": "Archivo no enviado"})
				}

				folderID := strings.TrimSpace(c.FormValue("folder_id"))
				if folderID == "" {
					folderID = strings.TrimSpace(c.QueryParam("folder_id"))
				}

				meta, err := gdriveClient.UploadDocInFolder(c.Request().Context(), file, folderID)
				if err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{
						"message": fmt.Sprintf("Error subiendo a Drive: %v", err),
					})
				}

				return c.JSON(http.StatusCreated, meta)
			}, storageGuard)

			// Crear un nuevo Google Doc o Google Sheet en blanco dentro de la carpeta indicada (can_storage)
			docsGroup.POST("/create", func(c echo.Context) error {
				var req struct {
					Title    string `json:"title"`
					Type     string `json:"type"` // "doc" o "sheet"
					FolderID string `json:"folder_id"`
				}
				if err := c.Bind(&req); err != nil || strings.TrimSpace(req.Title) == "" {
					return c.JSON(http.StatusBadRequest, map[string]string{"message": "El título del documento es requerido"})
				}

				meta, err := gdriveClient.CreateBlankFile(c.Request().Context(), req.Title, req.Type, req.FolderID)
				if err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{
						"message": fmt.Sprintf("Error creando documento en Drive: %v", err),
					})
				}

				return c.JSON(http.StatusCreated, meta)
			}, storageGuard)

			// Rutas avanzadas de Docs: membretes, sangrías, márgenes, viñetas, estilos e imágenes
			routes.RegisterDocsAdvancedRoutes(docsGroup, gdriveClient, permStore)
		}

		// 4. Subgrupo de Hojas de Cálculo (/api/sheets): tabs, fórmulas, filas/columnas completas, filtros, imágenes
		routes.RegisterSheetsRoutes(api, gdriveClient, permStore)

		// 5. Utilidad de subida de imágenes con permisos públicos para Docs y Sheets
		routes.RegisterImageRoutes(api, gdriveClient, permStore)

		// 6. Checklist y Gestión de Permisos de Carpetas y Capacidades (/api/permissions y /api/users)
		routes.RegisterPermissionRoutes(api, gdriveClient, permStore, authSvc)
	}

	log.Printf("Servidor corriendo en el puerto :%s", appCfg.Port)
	e.Logger.Fatal(e.Start(":" + appCfg.Port))
}
