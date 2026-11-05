package gdrive

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/sheets/v4"
)

// TabInfo representa los metadatos de una hoja/pestaña dentro de una planilla de cálculo
type TabInfo struct {
	SheetID     int64  `json:"sheet_id"`
	Title       string `json:"title"`
	Index       int64  `json:"index"`
	RowCount    int64  `json:"row_count"`
	ColumnCount int64  `json:"column_count"`
	Hidden      bool   `json:"hidden"`
}

// SheetValuesResponse representa los datos de lectura de celdas o rangos
type SheetValuesResponse struct {
	SheetName    string     `json:"sheet_name"`
	Range        string     `json:"range"`
	RenderOption string     `json:"render_option"`
	Rows         [][]string `json:"rows"`
}

// FiltersResponse agrupa información de filtros básicos y vistas de filtro
type FiltersResponse struct {
	BasicFilters []BasicFilterInfo `json:"basic_filters"`
	FilterViews  []FilterViewInfo  `json:"filter_views"`
}

type BasicFilterInfo struct {
	SheetID          int64 `json:"sheet_id"`
	StartRowIndex    int64 `json:"start_row_index"`
	EndRowIndex      int64 `json:"end_row_index"`
	StartColumnIndex int64 `json:"start_column_index"`
	EndColumnIndex   int64 `json:"end_column_index"`
}

type FilterViewInfo struct {
	FilterViewID     int64  `json:"filter_view_id"`
	Title            string `json:"title"`
	SheetID          int64  `json:"sheet_id"`
	StartRowIndex    int64  `json:"start_row_index"`
	EndRowIndex      int64  `json:"end_row_index"`
	StartColumnIndex int64  `json:"start_column_index"`
	EndColumnIndex   int64  `json:"end_column_index"`
}

// =========================================================================
// 1. GESTIÓN DE TABS (PESTAÑAS / HOJAS)
// =========================================================================

// ListTabs lista todas las pestañas de la hoja de cálculo
func (c *Client) ListTabs(ctx context.Context, spreadsheetID string) ([]TabInfo, error) {
	meta, err := c.sheets.Spreadsheets.Get(spreadsheetID).Fields("sheets(properties)").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("error obteniendo pestañas: %w", err)
	}

	tabs := make([]TabInfo, 0, len(meta.Sheets))
	for _, s := range meta.Sheets {
		p := s.Properties
		var rowCount, colCount int64
		if p.GridProperties != nil {
			rowCount = p.GridProperties.RowCount
			colCount = p.GridProperties.ColumnCount
		}
		tabs = append(tabs, TabInfo{
			SheetID:     p.SheetId,
			Title:       p.Title,
			Index:       p.Index,
			RowCount:    rowCount,
			ColumnCount: colCount,
			Hidden:      p.Hidden,
		})
	}
	return tabs, nil
}

// AddTab crea una nueva pestaña
func (c *Client) AddTab(ctx context.Context, spreadsheetID string, title string, rowCount int64, colCount int64) (*TabInfo, error) {
	if rowCount <= 0 {
		rowCount = 1000
	}
	if colCount <= 0 {
		colCount = 26
	}

	req := &sheets.Request{
		AddSheet: &sheets.AddSheetRequest{
			Properties: &sheets.SheetProperties{
				Title: title,
				GridProperties: &sheets.GridProperties{
					RowCount:    rowCount,
					ColumnCount: colCount,
				},
			},
		},
	}

	resp, err := c.sheets.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("error creando pestaña: %w", err)
	}

	if len(resp.Replies) == 0 || resp.Replies[0].AddSheet == nil {
		return nil, fmt.Errorf("no se recibió respuesta de creación de pestaña")
	}

	p := resp.Replies[0].AddSheet.Properties
	return &TabInfo{
		SheetID:     p.SheetId,
		Title:       p.Title,
		Index:       p.Index,
		RowCount:    p.GridProperties.RowCount,
		ColumnCount: p.GridProperties.ColumnCount,
		Hidden:      p.Hidden,
	}, nil
}

// RenameTab renombra una pestaña por su SheetID
func (c *Client) RenameTab(ctx context.Context, spreadsheetID string, sheetID int64, newTitle string) error {
	req := &sheets.Request{
		UpdateSheetProperties: &sheets.UpdateSheetPropertiesRequest{
			Properties: &sheets.SheetProperties{
				SheetId: sheetID,
				Title:   newTitle,
			},
			Fields: "title",
		},
	}

	_, err := c.sheets.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error renombrando pestaña: %w", err)
	}
	return nil
}

// DeleteTab elimina una pestaña por su SheetID
func (c *Client) DeleteTab(ctx context.Context, spreadsheetID string, sheetID int64) error {
	req := &sheets.Request{
		DeleteSheet: &sheets.DeleteSheetRequest{
			SheetId: sheetID,
		},
	}

	_, err := c.sheets.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error eliminando pestaña: %w", err)
	}
	return nil
}

// DuplicateTab duplica una pestaña existente
func (c *Client) DuplicateTab(ctx context.Context, spreadsheetID string, sheetID int64, newTitle string) (*TabInfo, error) {
	req := &sheets.Request{
		DuplicateSheet: &sheets.DuplicateSheetRequest{
			SourceSheetId: sheetID,
			NewSheetName:  newTitle,
		},
	}

	resp, err := c.sheets.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("error duplicando pestaña: %w", err)
	}

	if len(resp.Replies) == 0 || resp.Replies[0].DuplicateSheet == nil {
		return nil, fmt.Errorf("no se recibió respuesta de duplicación de pestaña")
	}

	p := resp.Replies[0].DuplicateSheet.Properties
	var rowCount, colCount int64
	if p.GridProperties != nil {
		rowCount = p.GridProperties.RowCount
		colCount = p.GridProperties.ColumnCount
	}

	return &TabInfo{
		SheetID:     p.SheetId,
		Title:       p.Title,
		Index:       p.Index,
		RowCount:    rowCount,
		ColumnCount: colCount,
		Hidden:      p.Hidden,
	}, nil
}

// =========================================================================
// 2. FORMULAS Y LECTURA/ESCRITURA AVANZADA DE CELDAS
// =========================================================================

// GetSheetValues obtiene datos de celdas permitiendo elegir si se leen fórmulas o valores calculados
// renderOption: "FORMULA", "FORMATTED_VALUE", "UNFORMATTED_VALUE"
func (c *Client) GetSheetValues(ctx context.Context, spreadsheetID, sheetName, rangeStr, renderOption string) (*SheetValuesResponse, error) {
	if renderOption == "" {
		renderOption = "FORMATTED_VALUE"
	}

	if sheetName == "" {
		tabs, err := c.ListTabs(ctx, spreadsheetID)
		if err != nil {
			return nil, err
		}
		if len(tabs) > 0 {
			sheetName = tabs[0].Title
		} else {
			sheetName = "Sheet1"
		}
	}

	fullRange := sheetName
	if rangeStr != "" {
		fullRange = fmt.Sprintf("'%s'!%s", sheetName, rangeStr)
	} else {
		fullRange = fmt.Sprintf("'%s'!A:ZZ", sheetName)
	}

	call := c.sheets.Spreadsheets.Values.Get(spreadsheetID, fullRange).ValueRenderOption(renderOption).Context(ctx)
	resp, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("error leyendo celdas: %w", err)
	}

	matrix := make([][]string, 0, len(resp.Values))
	for _, row := range resp.Values {
		strRow := make([]string, len(row))
		for i, col := range row {
			strRow[i] = fmt.Sprintf("%v", col)
		}
		matrix = append(matrix, strRow)
	}

	return &SheetValuesResponse{
		SheetName:    sheetName,
		Range:        fullRange,
		RenderOption: renderOption,
		Rows:         matrix,
	}, nil
}

// SetCellFormula asigna una fórmula matemática o lógica en una celda específica
func (c *Client) SetCellFormula(ctx context.Context, spreadsheetID, sheetName, cell, formula string) error {
	if !strings.HasPrefix(formula, "=") {
		formula = "=" + formula
	}
	return c.SetCellValue(ctx, spreadsheetID, sheetName, cell, formula)
}

// SetCellValue asigna cualquier valor (texto, número o fórmula con '=') en una celda específica
func (c *Client) SetCellValue(ctx context.Context, spreadsheetID, sheetName, cell, value string) error {
	if sheetName == "" {
		tabs, err := c.ListTabs(ctx, spreadsheetID)
		if err == nil && len(tabs) > 0 {
			sheetName = tabs[0].Title
		} else {
			sheetName = "Sheet1"
		}
	}
	targetRange := fmt.Sprintf("'%s'!%s", sheetName, cell)

	vr := &sheets.ValueRange{
		Values: [][]interface{}{{value}},
	}

	_, err := c.sheets.Spreadsheets.Values.Update(spreadsheetID, targetRange, vr).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("error actualizando la celda %s: %w", cell, err)
	}
	return nil
}

// =========================================================================
// 3. SELECCIÓN COMPLETA DE COLUMNA Y SELECCIÓN COMPLETA DE FILA
// =========================================================================

// GetColumn obtiene todos los valores de una columna completa (ej: "A", "C", "AA")
func (c *Client) GetColumn(ctx context.Context, spreadsheetID, sheetName, colLetter, renderOption string) ([]string, error) {
	colLetter = strings.ToUpper(strings.TrimSpace(colLetter))
	if colLetter == "" {
		return nil, fmt.Errorf("columna requerida (ej: 'A')")
	}

	rangeStr := fmt.Sprintf("%s:%s", colLetter, colLetter)
	res, err := c.GetSheetValues(ctx, spreadsheetID, sheetName, rangeStr, renderOption)
	if err != nil {
		return nil, err
	}

	columnValues := make([]string, 0, len(res.Rows))
	for _, row := range res.Rows {
		if len(row) > 0 {
			columnValues = append(columnValues, row[0])
		} else {
			columnValues = append(columnValues, "")
		}
	}
	return columnValues, nil
}

// UpdateColumn actualiza todos los valores de una columna completa
func (c *Client) UpdateColumn(ctx context.Context, spreadsheetID, sheetName, colLetter string, values []string) error {
	colLetter = strings.ToUpper(strings.TrimSpace(colLetter))
	if colLetter == "" {
		return fmt.Errorf("columna requerida (ej: 'A')")
	}
	if sheetName == "" {
		sheetName = "Sheet1"
	}

	matrix := make([][]interface{}, len(values))
	for i, v := range values {
		matrix[i] = []interface{}{v}
	}

	targetRange := fmt.Sprintf("'%s'!%s1", sheetName, colLetter)
	vr := &sheets.ValueRange{Values: matrix}

	_, err := c.sheets.Spreadsheets.Values.Update(spreadsheetID, targetRange, vr).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("error actualizando columna %s: %w", colLetter, err)
	}
	return nil
}

// GetRow obtiene todos los valores de una fila completa (ej: fila 1, fila 5)
func (c *Client) GetRow(ctx context.Context, spreadsheetID, sheetName string, rowNumber int, renderOption string) ([]string, error) {
	if rowNumber <= 0 {
		return nil, fmt.Errorf("número de fila debe ser mayor a 0")
	}

	rangeStr := fmt.Sprintf("%d:%d", rowNumber, rowNumber)
	res, err := c.GetSheetValues(ctx, spreadsheetID, sheetName, rangeStr, renderOption)
	if err != nil {
		return nil, err
	}

	if len(res.Rows) > 0 {
		return res.Rows[0], nil
	}
	return []string{}, nil
}

// UpdateRow actualiza todos los valores de una fila completa
func (c *Client) UpdateRow(ctx context.Context, spreadsheetID, sheetName string, rowNumber int, values []string) error {
	if rowNumber <= 0 {
		return fmt.Errorf("número de fila debe ser mayor a 0")
	}
	if sheetName == "" {
		sheetName = "Sheet1"
	}

	rowVals := make([]interface{}, len(values))
	for i, v := range values {
		rowVals[i] = v
	}

	targetRange := fmt.Sprintf("'%s'!A%d", sheetName, rowNumber)
	vr := &sheets.ValueRange{Values: [][]interface{}{rowVals}}

	_, err := c.sheets.Spreadsheets.Values.Update(spreadsheetID, targetRange, vr).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("error actualizando fila %d: %w", rowNumber, err)
	}
	return nil
}

// ManageDimensions inserta o elimina filas o columnas completas
// dimension: "ROWS" o "COLUMNS"
// operation: "INSERT" o "DELETE"
// startIndex, endIndex son basados en 0
func (c *Client) ManageDimensions(ctx context.Context, spreadsheetID string, sheetID int64, dimension, operation string, startIndex, endIndex int64, inheritFromBefore bool) error {
	dimension = strings.ToUpper(dimension)
	if dimension != "ROWS" && dimension != "COLUMNS" {
		return fmt.Errorf("dimension inválida: debe ser 'ROWS' o 'COLUMNS'")
	}

	var req *sheets.Request
	switch strings.ToUpper(operation) {
	case "INSERT":
		req = &sheets.Request{
			InsertDimension: &sheets.InsertDimensionRequest{
				Range: &sheets.DimensionRange{
					SheetId:    sheetID,
					Dimension:  dimension,
					StartIndex: startIndex,
					EndIndex:   endIndex,
				},
				InheritFromBefore: inheritFromBefore,
			},
		}
	case "DELETE":
		req = &sheets.Request{
			DeleteDimension: &sheets.DeleteDimensionRequest{
				Range: &sheets.DimensionRange{
					SheetId:    sheetID,
					Dimension:  dimension,
					StartIndex: startIndex,
					EndIndex:   endIndex,
				},
			},
		}
	default:
		return fmt.Errorf("operación inválida: debe ser 'INSERT' o 'DELETE'")
	}

	_, err := c.sheets.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error modificando dimensiones: %w", err)
	}
	return nil
}

// AutoResizeDimensions ajusta automáticamente el ancho de columnas o altura de filas según el contenido
func (c *Client) AutoResizeDimensions(ctx context.Context, spreadsheetID string, sheetID int64, dimension string, startIndex, endIndex int64) error {
	dimension = strings.ToUpper(dimension)
	if dimension != "ROWS" && dimension != "COLUMNS" {
		return fmt.Errorf("dimension inválida: debe ser 'ROWS' o 'COLUMNS'")
	}

	req := &sheets.Request{
		AutoResizeDimensions: &sheets.AutoResizeDimensionsRequest{
			Dimensions: &sheets.DimensionRange{
				SheetId:    sheetID,
				Dimension:  dimension,
				StartIndex: startIndex,
				EndIndex:   endIndex,
			},
		},
	}

	_, err := c.sheets.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error auto-ajustando dimensiones: %w", err)
	}
	return nil
}

// =========================================================================
// 4. FILTROS (BASIC FILTERS Y FILTER VIEWS)
// =========================================================================

// GetFilters obtiene los filtros activos en la planilla
func (c *Client) GetFilters(ctx context.Context, spreadsheetID string) (*FiltersResponse, error) {
	meta, err := c.sheets.Spreadsheets.Get(spreadsheetID).
		Fields("sheets(properties(sheetId),basicFilter,filterViews)").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("error obteniendo filtros: %w", err)
	}

	resp := &FiltersResponse{
		BasicFilters: make([]BasicFilterInfo, 0),
		FilterViews:  make([]FilterViewInfo, 0),
	}

	for _, s := range meta.Sheets {
		if s.BasicFilter != nil && s.BasicFilter.Range != nil {
			r := s.BasicFilter.Range
			resp.BasicFilters = append(resp.BasicFilters, BasicFilterInfo{
				SheetID:          r.SheetId,
				StartRowIndex:    r.StartRowIndex,
				EndRowIndex:      r.EndRowIndex,
				StartColumnIndex: r.StartColumnIndex,
				EndColumnIndex:   r.EndColumnIndex,
			})
		}

		for _, fv := range s.FilterViews {
			if fv.Range != nil {
				resp.FilterViews = append(resp.FilterViews, FilterViewInfo{
					FilterViewID:     fv.FilterViewId,
					Title:            fv.Title,
					SheetID:          fv.Range.SheetId,
					StartRowIndex:    fv.Range.StartRowIndex,
					EndRowIndex:      fv.Range.EndRowIndex,
					StartColumnIndex: fv.Range.StartColumnIndex,
					EndColumnIndex:   fv.Range.EndColumnIndex,
				})
			}
		}
	}

	return resp, nil
}

// SetBasicFilter activa un filtro básico en el rango indicado
func (c *Client) SetBasicFilter(ctx context.Context, spreadsheetID string, sheetID int64, startRow, endRow, startCol, endCol int64) error {
	req := &sheets.Request{
		SetBasicFilter: &sheets.SetBasicFilterRequest{
			Filter: &sheets.BasicFilter{
				Range: &sheets.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    startRow,
					EndRowIndex:      endRow,
					StartColumnIndex: startCol,
					EndColumnIndex:   endCol,
				},
			},
		},
	}

	_, err := c.sheets.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error configurando filtro básico: %w", err)
	}
	return nil
}

// ClearBasicFilter elimina el filtro básico de una pestaña
func (c *Client) ClearBasicFilter(ctx context.Context, spreadsheetID string, sheetID int64) error {
	req := &sheets.Request{
		ClearBasicFilter: &sheets.ClearBasicFilterRequest{
			SheetId: sheetID,
		},
	}

	_, err := c.sheets.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error limpiando filtro básico: %w", err)
	}
	return nil
}

// AddFilterView crea una nueva vista de filtro personalizada
func (c *Client) AddFilterView(ctx context.Context, spreadsheetID string, sheetID int64, title string, startRow, endRow, startCol, endCol int64) (int64, error) {
	req := &sheets.Request{
		AddFilterView: &sheets.AddFilterViewRequest{
			Filter: &sheets.FilterView{
				Title: title,
				Range: &sheets.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    startRow,
					EndRowIndex:      endRow,
					StartColumnIndex: startCol,
					EndColumnIndex:   endCol,
				},
			},
		},
	}

	resp, err := c.sheets.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return 0, fmt.Errorf("error agregando vista de filtro: %w", err)
	}

	if len(resp.Replies) == 0 || resp.Replies[0].AddFilterView == nil {
		return 0, fmt.Errorf("no se recibió respuesta al agregar vista de filtro")
	}

	return resp.Replies[0].AddFilterView.Filter.FilterViewId, nil
}

// DeleteFilterView elimina una vista de filtro
func (c *Client) DeleteFilterView(ctx context.Context, spreadsheetID string, filterViewID int64) error {
	req := &sheets.Request{
		DeleteFilterView: &sheets.DeleteFilterViewRequest{
			FilterId: filterViewID,
		},
	}

	_, err := c.sheets.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error eliminando vista de filtro: %w", err)
	}
	return nil
}

// =========================================================================
// 5. INSERTAR IMÁGENES EN GOOGLE SHEETS
// =========================================================================

// InsertImageToCell inserta una imagen en una celda mediante la función nativa =IMAGE("url", modo)
// Modos:
// 1 = Redimensiona para que quepa en la celda (mantiene relación de aspecto)
// 2 = Estira o comprime para ajustarse exactamente al tamaño de la celda
// 3 = Mantiene tamaño original
// 4 = Tamaño personalizado (requiere width y height)
func (c *Client) InsertImageToCell(ctx context.Context, spreadsheetID, sheetName, cell, imageURL string, mode int, height, width int) error {
	if mode <= 0 {
		mode = 1
	}

	var formula string
	if mode == 4 && height > 0 && width > 0 {
		formula = fmt.Sprintf(`=IMAGE("%s", 4, %d, %d)`, imageURL, height, width)
	} else if mode != 1 {
		formula = fmt.Sprintf(`=IMAGE("%s", %d)`, imageURL, mode)
	} else {
		formula = fmt.Sprintf(`=IMAGE("%s")`, imageURL)
	}

	return c.SetCellFormula(ctx, spreadsheetID, sheetName, cell, formula)
}

