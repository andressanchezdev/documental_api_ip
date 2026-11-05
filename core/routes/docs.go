package routes

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"doc-api/core/auth"
	"doc-api/core/platform/gdrive"
)

// RegisterDocsAdvancedRoutes registra las rutas de Docs (membretes, sangrías, márgenes, imágenes, etc.)
func RegisterDocsAdvancedRoutes(docsGroup *echo.Group, client *gdrive.Client, permChecker auth.PermissionChecker) {
	editGuard := auth.RequireEditPermission(permChecker)

	// ==========================================================
	// 1. ESTRUCTURA Y METADATOS DEL DOCUMENTO
	// ==========================================================

	docsGroup.GET("/:id/structure", func(c echo.Context) error {
		fileID := strings.TrimSpace(c.Param("id"))
		structure, err := client.GetDocumentStructure(c.Request().Context(), fileID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, structure)
	})

	// ==========================================================
	// 2. MEMBRETES (ENCABEZADOS Y PIES DE PÁGINA)
	// ==========================================================

	// Crear encabezado / membrete
	docsGroup.POST("/:id/headers", func(c echo.Context) error {
		fileID := strings.TrimSpace(c.Param("id"))
		var req struct {
			HeaderType string `json:"header_type"`
		}
		_ = c.Bind(&req)

		// Si ya existe un encabezado en el documento, reutilizar su ID para evitar error de duplicado en Google Docs API
		structure, err := client.GetDocumentStructure(c.Request().Context(), fileID)
		if err == nil && structure != nil {
			if structure.DocumentStyle.DefaultHeaderID != "" {
				return c.JSON(http.StatusOK, map[string]string{
					"header_id": structure.DocumentStyle.DefaultHeaderID,
					"message":   "Encabezado existente reutilizado",
				})
			}
			for existingID := range structure.Headers {
				return c.JSON(http.StatusOK, map[string]string{
					"header_id": existingID,
					"message":   "Encabezado existente reutilizado",
				})
			}
		}

		headerID, err := client.CreateHeader(c.Request().Context(), fileID, req.HeaderType)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusCreated, map[string]string{
			"header_id": headerID,
			"message":   "Encabezado creado exitosamente",
		})
	}, editGuard)

	// Eliminar encabezado
	docsGroup.DELETE("/:id/headers/:headerId", func(c echo.Context) error {
		fileID := strings.TrimSpace(c.Param("id"))
		headerID := strings.TrimSpace(c.Param("headerId"))
		if err := client.DeleteHeader(c.Request().Context(), fileID, headerID); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Encabezado eliminado exitosamente"})
	}, editGuard)

	// Configurar contenido del membrete (texto, logo de imagen y alineación)
	docsGroup.POST("/:id/headers/:headerId/content", func(c echo.Context) error {
		fileID := strings.TrimSpace(c.Param("id"))
		headerID := strings.TrimSpace(c.Param("headerId"))

		var req struct {
			Text        string  `json:"text"`
			ImageURL    string  `json:"image_url"`
			ImageWidth  float64 `json:"image_width"`
			ImageHeight float64 `json:"image_height"`
			Alignment   string  `json:"alignment"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Datos inválidos"})
		}

		if err := client.SetHeaderContent(c.Request().Context(), fileID, headerID, req.Text, req.ImageURL, req.ImageWidth, req.ImageHeight, req.Alignment); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Contenido del membrete configurado correctamente"})
	}, editGuard)

	// Crear pie de página
	docsGroup.POST("/:id/footers", func(c echo.Context) error {
		fileID := strings.TrimSpace(c.Param("id"))
		var req struct {
			FooterType string `json:"footer_type"`
		}
		_ = c.Bind(&req)

		structure, err := client.GetDocumentStructure(c.Request().Context(), fileID)
		if err == nil && structure != nil {
			if structure.DocumentStyle.DefaultFooterID != "" {
				return c.JSON(http.StatusOK, map[string]string{
					"footer_id": structure.DocumentStyle.DefaultFooterID,
					"message":   "Pie de página existente reutilizado",
				})
			}
			for existingID := range structure.Footers {
				return c.JSON(http.StatusOK, map[string]string{
					"footer_id": existingID,
					"message":   "Pie de página existente reutilizado",
				})
			}
		}

		footerID, err := client.CreateFooter(c.Request().Context(), fileID, req.FooterType)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusCreated, map[string]string{
			"footer_id": footerID,
			"message":   "Pie de página creado exitosamente",
		})
	}, editGuard)

	// Eliminar pie de página
	docsGroup.DELETE("/:id/footers/:footerId", func(c echo.Context) error {
		fileID := strings.TrimSpace(c.Param("id"))
		footerID := strings.TrimSpace(c.Param("footerId"))
		if err := client.DeleteFooter(c.Request().Context(), fileID, footerID); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Pie de página eliminado exitosamente"})
	}, editGuard)

	// Configurar contenido del pie de página
	docsGroup.POST("/:id/footers/:footerId/content", func(c echo.Context) error {
		fileID := strings.TrimSpace(c.Param("id"))
		footerID := strings.TrimSpace(c.Param("footerId"))

		var req struct {
			Text      string `json:"text"`
			Alignment string `json:"alignment"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Datos inválidos"})
		}

		if err := client.SetFooterContent(c.Request().Context(), fileID, footerID, req.Text, req.Alignment); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Contenido del pie de página configurado"})
	}, editGuard)

	// ==========================================================
	// 3. SANGRÍAS, INTERLINEADO, ESPACIADOS Y MÁRGENES
	// ==========================================================

	// Configurar formato de párrafo (sangrías, alineación, interlineado, espaciados)
	docsGroup.PUT("/:id/paragraphs/format", func(c echo.Context) error {
		fileID := strings.TrimSpace(c.Param("id"))
		var req gdrive.ParagraphFormatOptions
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Datos inválidos"})
		}

		if err := client.FormatParagraph(c.Request().Context(), fileID, req); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Formato de párrafo y sangrías aplicados exitosamente"})
	}, editGuard)

	// Configurar márgenes de página del documento
	docsGroup.PUT("/:id/margins", func(c echo.Context) error {
		fileID := strings.TrimSpace(c.Param("id"))
		var req gdrive.MarginOptions
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Datos inválidos"})
		}

		if err := client.FormatDocumentMargins(c.Request().Context(), fileID, req); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Márgenes del documento actualizados exitosamente"})
	}, editGuard)

	// Configurar estilo de texto (negrita, cursiva, subrayado, fuente, tamaño, color)
	docsGroup.PUT("/:id/text/format", func(c echo.Context) error {
		fileID := strings.TrimSpace(c.Param("id"))
		var req gdrive.TextFormatOptions
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Datos inválidos"})
		}

		if err := client.FormatText(c.Request().Context(), fileID, req); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Estilo de texto aplicado correctamente"})
	}, editGuard)

	// Convertir a viñetas / numeración
	docsGroup.POST("/:id/bullets", func(c echo.Context) error {
		fileID := strings.TrimSpace(c.Param("id"))
		var req struct {
			SegmentID    string `json:"segment_id"`
			StartIndex   int64  `json:"start_index"`
			EndIndex     int64  `json:"end_index"`
			BulletPreset string `json:"bullet_preset"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Datos inválidos"})
		}

		if err := client.CreateParagraphBullets(c.Request().Context(), fileID, req.SegmentID, req.StartIndex, req.EndIndex, req.BulletPreset); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Viñetas aplicadas correctamente"})
	}, editGuard)

	// Quitar viñetas
	docsGroup.DELETE("/:id/bullets", func(c echo.Context) error {
		fileID := strings.TrimSpace(c.Param("id"))
		var req struct {
			SegmentID  string `json:"segment_id"`
			StartIndex int64  `json:"start_index"`
			EndIndex   int64  `json:"end_index"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Datos inválidos"})
		}

		if err := client.DeleteParagraphBullets(c.Request().Context(), fileID, req.SegmentID, req.StartIndex, req.EndIndex); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Viñetas eliminadas correctamente"})
	}, editGuard)

	// ==========================================================
	// 4. INSERTAR IMÁGENES EN GOOGLE DOCS
	// ==========================================================

	// Insertar imagen inline mediante URL
	docsGroup.POST("/:id/images/insert", func(c echo.Context) error {
		fileID := strings.TrimSpace(c.Param("id"))
		var req struct {
			ImageURL  string  `json:"image_url"`
			SegmentID string  `json:"segment_id"`
			Index     int64   `json:"index"`
			Width     float64 `json:"width"`
			Height    float64 `json:"height"`
		}
		if err := c.Bind(&req); err != nil || strings.TrimSpace(req.ImageURL) == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "image_url es requerido"})
		}

		if err := client.InsertInlineImage(c.Request().Context(), fileID, req.ImageURL, req.SegmentID, req.Index, req.Width, req.Height); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Imagen insertada exitosamente en el documento"})
	}, editGuard)

	// Subir archivo de imagen y luego insertarlo automáticamente en el documento
	docsGroup.POST("/:id/images/upload", func(c echo.Context) error {
		fileID := strings.TrimSpace(c.Param("id"))
		file, err := c.FormFile("image")
		if err != nil {
			file, err = c.FormFile("file")
		}
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Archivo de imagen no enviado"})
		}

		segmentID := c.FormValue("segment_id")
		index, _ := strconv.ParseInt(c.FormValue("index"), 10, 64)
		width, _ := strconv.ParseFloat(c.FormValue("width"), 64)
		height, _ := strconv.ParseFloat(c.FormValue("height"), 64)

		// 1. Subir imagen a Google Drive
		imgInfo, err := client.UploadPublicImage(c.Request().Context(), file)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": fmt.Sprintf("Error subiendo imagen: %v", err)})
		}

		// 2. Insertar en el Google Doc
		if err := client.InsertInlineImage(c.Request().Context(), fileID, imgInfo.URL, segmentID, index, width, height); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": fmt.Sprintf("Error insertando imagen en el documento: %v", err)})
		}

		return c.JSON(http.StatusCreated, map[string]interface{}{
			"message":    "Imagen subida e insertada exitosamente en el documento",
			"image_info": imgInfo,
		})
	}, editGuard)
}
