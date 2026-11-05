package gdrive

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/api/docs/v1"
)

// DocStructureResponse describe la estructura de un Google Doc incluyendo membretes y márgenes
type DocStructureResponse struct {
	DocumentID    string                 `json:"document_id"`
	Title         string                 `json:"title"`
	Headers       map[string]HeaderInfo  `json:"headers"`
	Footers       map[string]FooterInfo  `json:"footers"`
	DocumentStyle DocumentStyleSummary   `json:"document_style"`
	BodyEndIndex  int64                  `json:"body_end_index"`
}

type HeaderInfo struct {
	HeaderID string `json:"header_id"`
	Content  string `json:"content"`
}

type FooterInfo struct {
	FooterID string `json:"footer_id"`
	Content  string `json:"content"`
}

type DocumentStyleSummary struct {
	DefaultHeaderID string  `json:"default_header_id,omitempty"`
	DefaultFooterID string  `json:"default_footer_id,omitempty"`
	MarginTop       float64 `json:"margin_top"`
	MarginBottom    float64 `json:"margin_bottom"`
	MarginLeft      float64 `json:"margin_left"`
	MarginRight     float64 `json:"margin_right"`
	MarginHeader    float64 `json:"margin_header"`
	MarginFooter    float64 `json:"margin_footer"`
	PageWidth       float64 `json:"page_width"`
	PageHeight      float64 `json:"page_height"`
}

// ParagraphFormatOptions opciones para sangrías, alineación e interlineado
type ParagraphFormatOptions struct {
	SegmentID       string   `json:"segment_id"`       // Vacío para cuerpo, o ID del Header/Footer
	StartIndex      int64    `json:"start_index"`      // Inicio del rango (ej: 1)
	EndIndex        int64    `json:"end_index"`        // Fin del rango
	IndentStart     *float64 `json:"indent_start"`     // Sangría izquierda (en pt, ej: 36.0 = 0.5 pulgada)
	IndentEnd       *float64 `json:"indent_end"`       // Sangría derecha (en pt)
	IndentFirstLine *float64 `json:"indent_first_line"`// Sangría de primera línea (en pt)
	Alignment       string   `json:"alignment"`        // START, CENTER, END, JUSTIFIED
	LineSpacing     *float64 `json:"line_spacing"`     // Porcentaje (ej: 100.0 normal, 115.0, 150.0, 200.0 doble)
	SpaceAbove      *float64 `json:"space_above"`      // Espacio antes del párrafo en pt
	SpaceBelow      *float64 `json:"space_below"`      // Espacio después del párrafo en pt
	NamedStyleType  string   `json:"named_style_type"` // NORMAL_TEXT, TITLE, HEADING_1, HEADING_2, etc.
}

// MarginOptions opciones para los márgenes del documento (en puntos, 72 pt = 1 pulgada)
type MarginOptions struct {
	MarginTop    *float64 `json:"margin_top"`
	MarginBottom *float64 `json:"margin_bottom"`
	MarginLeft   *float64 `json:"margin_left"`
	MarginRight  *float64 `json:"margin_right"`
	MarginHeader *float64 `json:"margin_header"`
	MarginFooter *float64 `json:"margin_footer"`
}

// TextFormatOptions opciones para estilo de fuentes
type TextFormatOptions struct {
	SegmentID       string   `json:"segment_id"`
	StartIndex      int64    `json:"start_index"`
	EndIndex        int64    `json:"end_index"`
	Bold            *bool    `json:"bold"`
	Italic          *bool    `json:"italic"`
	Underline       *bool    `json:"underline"`
	FontSize        *float64 `json:"font_size"`         // Tamaño en pt
	FontFamily      string   `json:"font_family"`       // Ej: "Arial", "Roboto", "Times New Roman"
	ForegroundColor string   `json:"foreground_color"`  // Color HEX: ej "#FF0000" o "1A73E8"
}

// =========================================================================
// 1. ESTRUCTURA Y MEMBRETES (HEADERS & FOOTERS)
// =========================================================================

// GetDocumentStructure obtiene los encabezados, pies de página, estilos y dimensiones de un Google Doc
func (c *Client) GetDocumentStructure(ctx context.Context, docID string) (*DocStructureResponse, error) {
	doc, err := c.docs.Documents.Get(docID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("error obteniendo estructura del documento: %w", err)
	}

	headers := make(map[string]HeaderInfo)
	for id, h := range doc.Headers {
		var text strings.Builder
		for _, elem := range h.Content {
			if elem.Paragraph != nil {
				for _, pe := range elem.Paragraph.Elements {
					if pe.TextRun != nil {
						text.WriteString(pe.TextRun.Content)
					}
				}
			}
		}
		headers[id] = HeaderInfo{HeaderID: id, Content: text.String()}
	}

	footers := make(map[string]FooterInfo)
	for id, f := range doc.Footers {
		var text strings.Builder
		for _, elem := range f.Content {
			if elem.Paragraph != nil {
				for _, pe := range elem.Paragraph.Elements {
					if pe.TextRun != nil {
						text.WriteString(pe.TextRun.Content)
					}
				}
			}
		}
		footers[id] = FooterInfo{FooterID: id, Content: text.String()}
	}

	var styleSum DocumentStyleSummary
	if doc.DocumentStyle != nil {
		ds := doc.DocumentStyle
		styleSum.DefaultHeaderID = ds.DefaultHeaderId
		styleSum.DefaultFooterID = ds.DefaultFooterId
		if ds.MarginTop != nil {
			styleSum.MarginTop = ds.MarginTop.Magnitude
		}
		if ds.MarginBottom != nil {
			styleSum.MarginBottom = ds.MarginBottom.Magnitude
		}
		if ds.MarginLeft != nil {
			styleSum.MarginLeft = ds.MarginLeft.Magnitude
		}
		if ds.MarginRight != nil {
			styleSum.MarginRight = ds.MarginRight.Magnitude
		}
		if ds.MarginHeader != nil {
			styleSum.MarginHeader = ds.MarginHeader.Magnitude
		}
		if ds.MarginFooter != nil {
			styleSum.MarginFooter = ds.MarginFooter.Magnitude
		}
		if ds.PageSize != nil {
			if ds.PageSize.Width != nil {
				styleSum.PageWidth = ds.PageSize.Width.Magnitude
			}
			if ds.PageSize.Height != nil {
				styleSum.PageHeight = ds.PageSize.Height.Magnitude
			}
		}
	}

	var bodyEnd int64 = 1
	if doc.Body != nil && len(doc.Body.Content) > 0 {
		bodyEnd = doc.Body.Content[len(doc.Body.Content)-1].EndIndex
	}

	return &DocStructureResponse{
		DocumentID:    doc.DocumentId,
		Title:         doc.Title,
		Headers:       headers,
		Footers:       footers,
		DocumentStyle: styleSum,
		BodyEndIndex:  bodyEnd,
	}, nil
}

// CreateHeader crea un encabezado (membrete) en el documento
// headerType: "DEFAULT"
func (c *Client) CreateHeader(ctx context.Context, docID string, headerType string) (string, error) {
	if headerType == "" {
		headerType = "DEFAULT"
	}

	req := &docs.Request{
		CreateHeader: &docs.CreateHeaderRequest{
			Type: headerType,
		},
	}

	res, err := c.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("error creando encabezado: %w", err)
	}

	if len(res.Replies) == 0 || res.Replies[0].CreateHeader == nil {
		return "", fmt.Errorf("no se recibió respuesta al crear el encabezado")
	}

	return res.Replies[0].CreateHeader.HeaderId, nil
}

// DeleteHeader elimina un encabezado
func (c *Client) DeleteHeader(ctx context.Context, docID string, headerID string) error {
	req := &docs.Request{
		DeleteHeader: &docs.DeleteHeaderRequest{
			HeaderId: headerID,
		},
	}

	_, err := c.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error eliminando encabezado: %w", err)
	}
	return nil
}

// CreateFooter crea un pie de página en el documento
// footerType: "DEFAULT"
func (c *Client) CreateFooter(ctx context.Context, docID string, footerType string) (string, error) {
	if footerType == "" {
		footerType = "DEFAULT"
	}

	req := &docs.Request{
		CreateFooter: &docs.CreateFooterRequest{
			Type: footerType,
		},
	}

	res, err := c.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("error creando pie de página: %w", err)
	}

	if len(res.Replies) == 0 || res.Replies[0].CreateFooter == nil {
		return "", fmt.Errorf("no se recibió respuesta al crear el pie de página")
	}

	return res.Replies[0].CreateFooter.FooterId, nil
}

// DeleteFooter elimina un pie de página
func (c *Client) DeleteFooter(ctx context.Context, docID string, footerID string) error {
	req := &docs.Request{
		DeleteFooter: &docs.DeleteFooterRequest{
			FooterId: footerID,
		},
	}

	_, err := c.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error eliminando pie de página: %w", err)
	}
	return nil
}

// SetHeaderContent agrega contenido al membrete/encabezado (texto, logo de imagen y alineación)
func (c *Client) SetHeaderContent(ctx context.Context, docID, headerID, text, imageURL string, imgWidth, imgHeight float64, alignment string) error {
	requests := make([]*docs.Request, 0)

	// 1. Si hay imagen de logo para el membrete, insertarla
	if imageURL != "" {
		imgReq := &docs.InsertInlineImageRequest{
			EndOfSegmentLocation: &docs.EndOfSegmentLocation{
				SegmentId: headerID,
			},
			Uri: imageURL,
		}
		if imgWidth > 0 && imgHeight > 0 {
			imgReq.ObjectSize = &docs.Size{
				Width:  &docs.Dimension{Magnitude: imgWidth, Unit: "PT"},
				Height: &docs.Dimension{Magnitude: imgHeight, Unit: "PT"},
			}
		} else if imgWidth > 0 {
			imgReq.ObjectSize = &docs.Size{
				Width: &docs.Dimension{Magnitude: imgWidth, Unit: "PT"},
			}
		}
		requests = append(requests, &docs.Request{InsertInlineImage: imgReq})
	}

	// 2. Si hay texto de membrete
	if text != "" {
		if imageURL != "" && !strings.HasPrefix(text, "\n") {
			text = "\n" + text
		}
		requests = append(requests, &docs.Request{
			InsertText: &docs.InsertTextRequest{
				EndOfSegmentLocation: &docs.EndOfSegmentLocation{
					SegmentId: headerID,
				},
				Text: text,
			},
		})
	}

	// 3. Si hay alineación deseada
	if alignment != "" {
		requests = append(requests, &docs.Request{
			UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
				Range: &docs.Range{
					SegmentId:  headerID,
					StartIndex: 0,
					EndIndex:   1,
				},
				ParagraphStyle: &docs.ParagraphStyle{
					Alignment: strings.ToUpper(alignment),
				},
				Fields: "alignment",
			},
		})
	}

	if len(requests) == 0 {
		return nil
	}

	_, err := c.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: requests,
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error configurando contenido del encabezado: %w", err)
	}
	return nil
}

// SetFooterContent agrega contenido al pie de página (texto y alineación)
func (c *Client) SetFooterContent(ctx context.Context, docID, footerID, text, alignment string) error {
	requests := make([]*docs.Request, 0)

	if text != "" {
		requests = append(requests, &docs.Request{
			InsertText: &docs.InsertTextRequest{
				EndOfSegmentLocation: &docs.EndOfSegmentLocation{
					SegmentId: footerID,
				},
				Text: text,
			},
		})
	}

	if alignment != "" {
		requests = append(requests, &docs.Request{
			UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
				Range: &docs.Range{
					SegmentId:  footerID,
					StartIndex: 0,
					EndIndex:   1,
				},
				ParagraphStyle: &docs.ParagraphStyle{
					Alignment: strings.ToUpper(alignment),
				},
				Fields: "alignment",
			},
		})
	}

	if len(requests) == 0 {
		return nil
	}

	_, err := c.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: requests,
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error configurando contenido del pie de página: %w", err)
	}
	return nil
}

// =========================================================================
// 2. SANGRÍAS, INTERLINEADO, ESPACIADOS Y MÁRGENES
// =========================================================================

// FormatParagraph configura sangría izquierda, derecha, de primera línea, alineación, interlineado y espaciados
func (c *Client) FormatParagraph(ctx context.Context, docID string, opts ParagraphFormatOptions) error {
	pStyle := &docs.ParagraphStyle{}
	fieldList := make([]string, 0)

	if opts.IndentStart != nil {
		pStyle.IndentStart = &docs.Dimension{Magnitude: *opts.IndentStart, Unit: "PT"}
		fieldList = append(fieldList, "indentStart")
	}
	if opts.IndentEnd != nil {
		pStyle.IndentEnd = &docs.Dimension{Magnitude: *opts.IndentEnd, Unit: "PT"}
		fieldList = append(fieldList, "indentEnd")
	}
	if opts.IndentFirstLine != nil {
		pStyle.IndentFirstLine = &docs.Dimension{Magnitude: *opts.IndentFirstLine, Unit: "PT"}
		fieldList = append(fieldList, "indentFirstLine")
	}
	if opts.Alignment != "" {
		pStyle.Alignment = strings.ToUpper(opts.Alignment)
		fieldList = append(fieldList, "alignment")
	}
	if opts.LineSpacing != nil {
		pStyle.LineSpacing = *opts.LineSpacing
		fieldList = append(fieldList, "lineSpacing")
	}
	if opts.SpaceAbove != nil {
		pStyle.SpaceAbove = &docs.Dimension{Magnitude: *opts.SpaceAbove, Unit: "PT"}
		fieldList = append(fieldList, "spaceAbove")
	}
	if opts.SpaceBelow != nil {
		pStyle.SpaceBelow = &docs.Dimension{Magnitude: *opts.SpaceBelow, Unit: "PT"}
		fieldList = append(fieldList, "spaceBelow")
	}
	if opts.NamedStyleType != "" {
		pStyle.NamedStyleType = strings.ToUpper(opts.NamedStyleType)
		fieldList = append(fieldList, "namedStyleType")
	}

	if len(fieldList) == 0 {
		return fmt.Errorf("ningún campo de formato de párrafo especificado")
	}

	if opts.EndIndex <= opts.StartIndex {
		// Ajustar al menos a abarcar el segmento
		opts.EndIndex = opts.StartIndex + 1
	}

	req := &docs.Request{
		UpdateParagraphStyle: &docs.UpdateParagraphStyleRequest{
			Range: &docs.Range{
				SegmentId:  opts.SegmentID,
				StartIndex: opts.StartIndex,
				EndIndex:   opts.EndIndex,
			},
			ParagraphStyle: pStyle,
			Fields:         strings.Join(fieldList, ","),
		},
	}

	_, err := c.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error aplicando formato de párrafo: %w", err)
	}
	return nil
}

// FormatDocumentMargins configura los márgenes de página del documento (arriba, abajo, izq, der, membrete)
func (c *Client) FormatDocumentMargins(ctx context.Context, docID string, opts MarginOptions) error {
	docStyle := &docs.DocumentStyle{}
	fieldList := make([]string, 0)

	if opts.MarginTop != nil {
		docStyle.MarginTop = &docs.Dimension{Magnitude: *opts.MarginTop, Unit: "PT"}
		fieldList = append(fieldList, "marginTop")
	}
	if opts.MarginBottom != nil {
		docStyle.MarginBottom = &docs.Dimension{Magnitude: *opts.MarginBottom, Unit: "PT"}
		fieldList = append(fieldList, "marginBottom")
	}
	if opts.MarginLeft != nil {
		docStyle.MarginLeft = &docs.Dimension{Magnitude: *opts.MarginLeft, Unit: "PT"}
		fieldList = append(fieldList, "marginLeft")
	}
	if opts.MarginRight != nil {
		docStyle.MarginRight = &docs.Dimension{Magnitude: *opts.MarginRight, Unit: "PT"}
		fieldList = append(fieldList, "marginRight")
	}
	if opts.MarginHeader != nil {
		docStyle.MarginHeader = &docs.Dimension{Magnitude: *opts.MarginHeader, Unit: "PT"}
		fieldList = append(fieldList, "marginHeader")
	}
	if opts.MarginFooter != nil {
		docStyle.MarginFooter = &docs.Dimension{Magnitude: *opts.MarginFooter, Unit: "PT"}
		fieldList = append(fieldList, "marginFooter")
	}

	if len(fieldList) == 0 {
		return fmt.Errorf("ningún margen especificado")
	}

	req := &docs.Request{
		UpdateDocumentStyle: &docs.UpdateDocumentStyleRequest{
			DocumentStyle: docStyle,
			Fields:        strings.Join(fieldList, ","),
		},
	}

	_, err := c.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error aplicando márgenes del documento: %w", err)
	}
	return nil
}

// FormatText aplica estilo de caracteres (negrita, cursiva, subrayado, fuente, tamaño, color)
func (c *Client) FormatText(ctx context.Context, docID string, opts TextFormatOptions) error {
	textStyle := &docs.TextStyle{}
	fieldList := make([]string, 0)

	if opts.Bold != nil {
		textStyle.Bold = *opts.Bold
		fieldList = append(fieldList, "bold")
	}
	if opts.Italic != nil {
		textStyle.Italic = *opts.Italic
		fieldList = append(fieldList, "italic")
	}
	if opts.Underline != nil {
		textStyle.Underline = *opts.Underline
		fieldList = append(fieldList, "underline")
	}
	if opts.FontSize != nil {
		textStyle.FontSize = &docs.Dimension{Magnitude: *opts.FontSize, Unit: "PT"}
		fieldList = append(fieldList, "fontSize")
	}
	if opts.FontFamily != "" {
		textStyle.WeightedFontFamily = &docs.WeightedFontFamily{FontFamily: opts.FontFamily}
		fieldList = append(fieldList, "weightedFontFamily")
	}
	if opts.ForegroundColor != "" {
		colorHex := strings.TrimPrefix(opts.ForegroundColor, "#")
		if len(colorHex) == 6 {
			r, _ := strconv.ParseUint(colorHex[0:2], 16, 8)
			g, _ := strconv.ParseUint(colorHex[2:4], 16, 8)
			b, _ := strconv.ParseUint(colorHex[4:6], 16, 8)
			textStyle.ForegroundColor = &docs.OptionalColor{
				Color: &docs.Color{
					RgbColor: &docs.RgbColor{
						Red:   float64(r) / 255.0,
						Green: float64(g) / 255.0,
						Blue:  float64(b) / 255.0,
					},
				},
			}
			fieldList = append(fieldList, "foregroundColor")
		}
	}

	if len(fieldList) == 0 {
		return fmt.Errorf("ningún estilo de texto especificado")
	}

	if opts.EndIndex <= opts.StartIndex {
		opts.EndIndex = opts.StartIndex + 1
	}

	req := &docs.Request{
		UpdateTextStyle: &docs.UpdateTextStyleRequest{
			Range: &docs.Range{
				SegmentId:  opts.SegmentID,
				StartIndex: opts.StartIndex,
				EndIndex:   opts.EndIndex,
			},
			TextStyle: textStyle,
			Fields:    strings.Join(fieldList, ","),
		},
	}

	_, err := c.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error aplicando formato de texto: %w", err)
	}
	return nil
}

// CreateParagraphBullets convierte los párrafos del rango en una lista con viñetas o numeración
// preset sugerido: "BULLET_DISC_CIRCLE_SQUARE", "NUMBERED_DECIMAL_ALPHA_ROMAN"
func (c *Client) CreateParagraphBullets(ctx context.Context, docID, segmentID string, startIndex, endIndex int64, preset string) error {
	if preset == "" {
		preset = "BULLET_DISC_CIRCLE_SQUARE"
	}

	if endIndex <= startIndex {
		endIndex = startIndex + 1
	}

	req := &docs.Request{
		CreateParagraphBullets: &docs.CreateParagraphBulletsRequest{
			Range: &docs.Range{
				SegmentId:  segmentID,
				StartIndex: startIndex,
				EndIndex:   endIndex,
			},
			BulletPreset: preset,
		},
	}

	_, err := c.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error aplicando viñetas: %w", err)
	}
	return nil
}

// DeleteParagraphBullets elimina las viñetas en el rango
func (c *Client) DeleteParagraphBullets(ctx context.Context, docID, segmentID string, startIndex, endIndex int64) error {
	if endIndex <= startIndex {
		endIndex = startIndex + 1
	}

	req := &docs.Request{
		DeleteParagraphBullets: &docs.DeleteParagraphBulletsRequest{
			Range: &docs.Range{
				SegmentId:  segmentID,
				StartIndex: startIndex,
				EndIndex:   endIndex,
			},
		},
	}

	_, err := c.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error eliminando viñetas: %w", err)
	}
	return nil
}

// =========================================================================
// 3. INSERTAR IMÁGENES EN GOOGLE DOCS
// =========================================================================

// InsertInlineImage inserta una imagen inline en el documento (cuerpo o segmento de encabezado/pie)
// con tamaño personalizado opcional en pt
func (c *Client) InsertInlineImage(ctx context.Context, docID, imageURL, segmentID string, index int64, width, height float64) error {
	imgReq := &docs.InsertInlineImageRequest{
		Uri: imageURL,
	}

	if index > 0 {
		imgReq.Location = &docs.Location{
			SegmentId: segmentID,
			Index:     index,
		}
	} else {
		imgReq.EndOfSegmentLocation = &docs.EndOfSegmentLocation{
			SegmentId: segmentID,
		}
	}

	if width > 0 && height > 0 {
		imgReq.ObjectSize = &docs.Size{
			Width:  &docs.Dimension{Magnitude: width, Unit: "PT"},
			Height: &docs.Dimension{Magnitude: height, Unit: "PT"},
		}
	} else if width > 0 {
		imgReq.ObjectSize = &docs.Size{
			Width: &docs.Dimension{Magnitude: width, Unit: "PT"},
		}
	}

	req := &docs.Request{
		InsertInlineImage: imgReq,
	}

	_, err := c.docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{req},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error insertando imagen en documento: %w", err)
	}
	return nil
}

