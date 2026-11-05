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

// RegisterSheetsRoutes registra todas las rutas relacionadas a Google Sheets API
func RegisterSheetsRoutes(api *echo.Group, client *gdrive.Client, permChecker auth.PermissionChecker) {
	sheetsGroup := api.Group("/sheets")
	editGuard := auth.RequireEditPermission(permChecker)

	// ==========================================================
	// 1. TABS (PESTAÑAS)
	// ==========================================================

	// Listar todas las pestañas de una planilla
	sheetsGroup.GET("/:id/tabs", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		tabs, err := client.ListTabs(c.Request().Context(), spreadsheetID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, tabs)
	})

	// Crear nueva pestaña
	sheetsGroup.POST("/:id/tabs", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		var req struct {
			Title       string `json:"title"`
			RowCount    int64  `json:"row_count"`
			ColumnCount int64  `json:"column_count"`
		}
		if err := c.Bind(&req); err != nil || strings.TrimSpace(req.Title) == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "El título de la pestaña es requerido"})
		}

		tab, err := client.AddTab(c.Request().Context(), spreadsheetID, strings.TrimSpace(req.Title), req.RowCount, req.ColumnCount)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusCreated, tab)
	}, editGuard)

	// Renombrar pestaña
	sheetsGroup.PUT("/:id/tabs/:tabId", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		tabID, err := strconv.ParseInt(c.Param("tabId"), 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "tabId inválido"})
		}

		var req struct {
			Title string `json:"title"`
		}
		if err := c.Bind(&req); err != nil || strings.TrimSpace(req.Title) == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Nuevo título requerido"})
		}

		if err := client.RenameTab(c.Request().Context(), spreadsheetID, tabID, strings.TrimSpace(req.Title)); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Pestaña renombrada exitosamente"})
	}, editGuard)

	// Eliminar pestaña
	sheetsGroup.DELETE("/:id/tabs/:tabId", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		tabID, err := strconv.ParseInt(c.Param("tabId"), 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "tabId inválido"})
		}

		if err := client.DeleteTab(c.Request().Context(), spreadsheetID, tabID); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Pestaña eliminada exitosamente"})
	}, editGuard)

	// Duplicar pestaña
	sheetsGroup.POST("/:id/tabs/:tabId/duplicate", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		tabID, err := strconv.ParseInt(c.Param("tabId"), 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "tabId inválido"})
		}

		var req struct {
			NewTitle string `json:"new_title"`
		}
		_ = c.Bind(&req)

		tab, err := client.DuplicateTab(c.Request().Context(), spreadsheetID, tabID, strings.TrimSpace(req.NewTitle))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusCreated, tab)
	}, editGuard)

	// ==========================================================
	// 2. FÓRMULAS Y LECTURA/ESCRITURA AVANZADA DE VALORES
	// ==========================================================

	// Leer celdas con soporte de fórmulas o valores evaluados
	sheetsGroup.GET("/:id/values", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		sheetName := c.QueryParam("sheet_name")
		rangeStr := c.QueryParam("range")
		renderOption := c.QueryParam("render_option")

		data, err := client.GetSheetValues(c.Request().Context(), spreadsheetID, sheetName, rangeStr, renderOption)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, data)
	})

	// Asignar una fórmula o valor a una celda
	sheetsGroup.POST("/:id/formula", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		var req struct {
			SheetName string `json:"sheet_name"`
			Cell      string `json:"cell"`
			Formula   string `json:"formula"`
			Value     string `json:"value"`
		}
		if err := c.Bind(&req); err != nil || strings.TrimSpace(req.Cell) == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "La celda es requerida"})
		}

		val := req.Formula
		if val == "" {
			val = req.Value
		}

		if err := client.SetCellValue(c.Request().Context(), spreadsheetID, req.SheetName, req.Cell, val); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Celda actualizada correctamente"})
	}, editGuard)

	// Actualizar valor directo de una celda (texto, número o fórmula)
	sheetsGroup.PUT("/:id/cell", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		var req struct {
			SheetName string `json:"sheet_name"`
			Cell      string `json:"cell"`
			Value     string `json:"value"`
		}
		if err := c.Bind(&req); err != nil || strings.TrimSpace(req.Cell) == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "La celda es requerida"})
		}

		if err := client.SetCellValue(c.Request().Context(), spreadsheetID, req.SheetName, req.Cell, req.Value); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Celda guardada exitosamente"})
	}, editGuard)

	// ==========================================================
	// 3. SELECCIÓN COMPLETA DE COLUMNA Y FILA
	// ==========================================================

	// Obtener columna completa
	sheetsGroup.GET("/:id/columns/:col", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		colLetter := c.Param("col")
		sheetName := c.QueryParam("sheet_name")
		renderOption := c.QueryParam("render_option")

		vals, err := client.GetColumn(c.Request().Context(), spreadsheetID, sheetName, colLetter, renderOption)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"column": strings.ToUpper(colLetter),
			"values": vals,
		})
	})

	// Actualizar columna completa
	sheetsGroup.PUT("/:id/columns/:col", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		colLetter := c.Param("col")
		var req struct {
			SheetName string   `json:"sheet_name"`
			Values    []string `json:"values"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Datos inválidos"})
		}

		if err := client.UpdateColumn(c.Request().Context(), spreadsheetID, req.SheetName, colLetter, req.Values); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": fmt.Sprintf("Columna %s actualizada", strings.ToUpper(colLetter))})
	}, editGuard)

	// Obtener fila completa
	sheetsGroup.GET("/:id/rows/:row", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		rowNum, err := strconv.Atoi(c.Param("row"))
		if err != nil || rowNum <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Número de fila inválido"})
		}
		sheetName := c.QueryParam("sheet_name")
		renderOption := c.QueryParam("render_option")

		vals, err := client.GetRow(c.Request().Context(), spreadsheetID, sheetName, rowNum, renderOption)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"row":    rowNum,
			"values": vals,
		})
	})

	// Actualizar fila completa
	sheetsGroup.PUT("/:id/rows/:row", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		rowNum, err := strconv.Atoi(c.Param("row"))
		if err != nil || rowNum <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Número de fila inválido"})
		}
		var req struct {
			SheetName string   `json:"sheet_name"`
			Values    []string `json:"values"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Datos inválidos"})
		}

		if err := client.UpdateRow(c.Request().Context(), spreadsheetID, req.SheetName, rowNum, req.Values); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": fmt.Sprintf("Fila %d actualizada", rowNum)})
	}, editGuard)

	// Insertar o eliminar filas o columnas (dimensiones)
	sheetsGroup.POST("/:id/dimensions", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		var req struct {
			SheetID           int64  `json:"sheet_id"`
			Dimension         string `json:"dimension"`           // "ROWS" o "COLUMNS"
			Operation         string `json:"operation"`           // "INSERT" o "DELETE"
			StartIndex        int64  `json:"start_index"`         // 0-indexed
			EndIndex          int64  `json:"end_index"`           // 0-indexed
			InheritFromBefore bool   `json:"inherit_from_before"` // true para heredar formato
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Datos inválidos"})
		}

		if err := client.ManageDimensions(c.Request().Context(), spreadsheetID, req.SheetID, req.Dimension, req.Operation, req.StartIndex, req.EndIndex, req.InheritFromBefore); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Operación de dimensiones completada exitosamente"})
	}, editGuard)

	// Auto-ajustar ancho de columna o alto de fila
	sheetsGroup.POST("/:id/dimensions/resize", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		var req struct {
			SheetID    int64  `json:"sheet_id"`
			Dimension  string `json:"dimension"` // "ROWS" o "COLUMNS"
			StartIndex int64  `json:"start_index"`
			EndIndex   int64  `json:"end_index"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Datos inválidos"})
		}

		if err := client.AutoResizeDimensions(c.Request().Context(), spreadsheetID, req.SheetID, req.Dimension, req.StartIndex, req.EndIndex); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Dimensiones auto-ajustadas"})
	}, editGuard)

	// ==========================================================
	// 4. FILTROS
	// ==========================================================

	// Obtener filtros activos
	sheetsGroup.GET("/:id/filters", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		filters, err := client.GetFilters(c.Request().Context(), spreadsheetID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, filters)
	})

	// Activar filtro básico
	sheetsGroup.POST("/:id/filters/basic", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		var req struct {
			SheetID       int64 `json:"sheet_id"`
			StartRowIndex int64 `json:"start_row_index"`
			EndRowIndex   int64 `json:"end_row_index"`
			StartColIndex int64 `json:"start_col_index"`
			EndColIndex   int64 `json:"end_col_index"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Datos inválidos"})
		}

		if err := client.SetBasicFilter(c.Request().Context(), spreadsheetID, req.SheetID, req.StartRowIndex, req.EndRowIndex, req.StartColIndex, req.EndColIndex); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Filtro básico configurado"})
	}, editGuard)

	// Limpiar / remover filtro básico
	sheetsGroup.DELETE("/:id/filters/basic/:tabId", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		tabID, err := strconv.ParseInt(c.Param("tabId"), 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "tabId inválido"})
		}

		if err := client.ClearBasicFilter(c.Request().Context(), spreadsheetID, tabID); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Filtro básico removido"})
	}, editGuard)

	// Agregar vista de filtro personalizada
	sheetsGroup.POST("/:id/filters/views", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		var req struct {
			SheetID       int64  `json:"sheet_id"`
			Title         string `json:"title"`
			StartRowIndex int64  `json:"start_row_index"`
			EndRowIndex   int64  `json:"end_row_index"`
			StartColIndex int64  `json:"start_col_index"`
			EndColIndex   int64  `json:"end_col_index"`
		}
		if err := c.Bind(&req); err != nil || strings.TrimSpace(req.Title) == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Título y rango requeridos"})
		}

		viewID, err := client.AddFilterView(c.Request().Context(), spreadsheetID, req.SheetID, req.Title, req.StartRowIndex, req.EndRowIndex, req.StartColIndex, req.EndColIndex)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusCreated, map[string]interface{}{
			"filter_view_id": viewID,
			"title":          req.Title,
		})
	}, editGuard)

	// Eliminar vista de filtro
	sheetsGroup.DELETE("/:id/filters/views/:viewId", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		viewID, err := strconv.ParseInt(c.Param("viewId"), 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "viewId inválido"})
		}

		if err := client.DeleteFilterView(c.Request().Context(), spreadsheetID, viewID); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Vista de filtro eliminada"})
	}, editGuard)

	// ==========================================================
	// 5. INSERTAR IMÁGENES EN CELDAS DE SHEETS
	// ==========================================================

	// Insertar imagen mediante URL
	sheetsGroup.POST("/:id/images/insert", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		var req struct {
			SheetName string `json:"sheet_name"`
			Cell      string `json:"cell"`
			ImageURL  string `json:"image_url"`
			Mode      int    `json:"mode"`
			Height    int    `json:"height"`
			Width     int    `json:"width"`
		}
		if err := c.Bind(&req); err != nil || strings.TrimSpace(req.Cell) == "" || strings.TrimSpace(req.ImageURL) == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "cell e image_url son requeridos"})
		}

		if err := client.InsertImageToCell(c.Request().Context(), spreadsheetID, req.SheetName, req.Cell, req.ImageURL, req.Mode, req.Height, req.Width); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Imagen insertada exitosamente en la celda"})
	}, editGuard)

	// Subir archivo de imagen y luego insertarlo automáticamente en la celda
	sheetsGroup.POST("/:id/images/upload", func(c echo.Context) error {
		spreadsheetID := strings.TrimSpace(c.Param("id"))
		file, err := c.FormFile("image")
		if err != nil {
			file, err = c.FormFile("file")
		}
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "Archivo de imagen no enviado"})
		}

		cell := c.FormValue("cell")
		if strings.TrimSpace(cell) == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "El campo 'cell' es requerido (ej: 'B2')"})
		}

		sheetName := c.FormValue("sheet_name")
		mode, _ := strconv.Atoi(c.FormValue("mode"))
		height, _ := strconv.Atoi(c.FormValue("height"))
		width, _ := strconv.Atoi(c.FormValue("width"))

		// 1. Subir imagen a Google Drive
		imgInfo, err := client.UploadPublicImage(c.Request().Context(), file)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": fmt.Sprintf("Error subiendo imagen: %v", err)})
		}

		// 2. Insertar en la celda especificada
		if err := client.InsertImageToCell(c.Request().Context(), spreadsheetID, sheetName, cell, imgInfo.URL, mode, height, width); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": fmt.Sprintf("Error insertando fórmula de imagen: %v", err)})
		}

		return c.JSON(http.StatusCreated, map[string]interface{}{
			"message":    "Imagen subida e insertada exitosamente en la celda",
			"image_info": imgInfo,
			"cell":       cell,
		})
	}, editGuard)
}
