package gdrive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type Client struct {
	drive  *drive.Service
	docs   *docs.Service
	sheets *sheets.Service
}

// DriveFileMetadata representa información completa de un archivo o carpeta en Drive
type DriveFileMetadata struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Title     string    `json:"title"`
	MimeType  string    `json:"mime_type"`
	Parents   []string  `json:"parents"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StorageQuota representa el estado del almacenamiento en Drive
type StorageQuota struct {
	Limit int64 `json:"limit"`
	Usage int64 `json:"usage"`
}

type SheetData struct {
	SheetName string     `json:"sheet_name"`
	Rows      [][]string `json:"rows"`
}

func (c *Client) GetStorageQuota(ctx context.Context) (*StorageQuota, error) {
	about, err := c.drive.About.Get().Fields("storageQuota").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("error obteniendo cuota de Drive: %w", err)
	}

	return &StorageQuota{
		Limit: about.StorageQuota.Limit,
		Usage: about.StorageQuota.Usage,
	}, nil
}

func (c *Client) GetSheetData(ctx context.Context, spreadsheetID string) (*SheetData, error) {
	meta, err := c.sheets.Spreadsheets.Get(spreadsheetID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("error leyendo metadatos de la hoja: %w", err)
	}

	if len(meta.Sheets) == 0 {
		return &SheetData{Rows: [][]string{}}, nil
	}

	firstSheetTitle := meta.Sheets[0].Properties.Title

	// Rango amplio: desde la columna A hasta ZZ sin límite estricto de filas
	readRange := fmt.Sprintf("'%s'!A:ZZ", firstSheetTitle)

	resp, err := c.sheets.Spreadsheets.Values.Get(spreadsheetID, readRange).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("error leyendo celdas: %w", err)
	}

	matrix := make([][]string, 0)
	for _, row := range resp.Values {
		strRow := make([]string, len(row))
		for i, col := range row {
			strRow[i] = fmt.Sprintf("%v", col)
		}
		matrix = append(matrix, strRow)
	}

	return &SheetData{
		SheetName: firstSheetTitle,
		Rows:      matrix,
	}, nil
}

// UpdateSheetData sobreescribe las celdas editadas
func (c *Client) UpdateSheetData(ctx context.Context, spreadsheetID string, sheetName string, rows [][]string) error {
	if sheetName == "" {
		meta, err := c.sheets.Spreadsheets.Get(spreadsheetID).Context(ctx).Do()
		if err == nil && len(meta.Sheets) > 0 {
			sheetName = meta.Sheets[0].Properties.Title
		} else {
			sheetName = "Sheet1"
		}
	}

	// 1. Limpiar el contenido anterior para que celdas borradas no queden residuales
	clearRange := fmt.Sprintf("'%s'!A:ZZ", sheetName)
	_, err := c.sheets.Spreadsheets.Values.Clear(spreadsheetID, clearRange, &sheets.ClearValuesRequest{}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error limpiando rango anterior: %w", err)
	}

	// 2. Preparar los nuevos valores
	var values [][]interface{}
	for _, row := range rows {
		rowVals := make([]interface{}, len(row))
		for i, cell := range row {
			rowVals[i] = cell
		}
		values = append(values, rowVals)
	}

	vr := &sheets.ValueRange{
		Values: values,
	}

	// 3. Escribir desde la celda A1 hacia adelante
	writeRange := fmt.Sprintf("'%s'!A1", sheetName)
	_, err = c.sheets.Spreadsheets.Values.Update(spreadsheetID, writeRange, vr).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()

	if err != nil {
		return fmt.Errorf("error actualizando celdas en Sheets: %w", err)
	}

	return nil
}

// ListDocs busca documentos existentes en Google Drive (Google Docs y Sheets) incluyendo sus carpetas contenedoras (parents)
func (c *Client) ListDocs(ctx context.Context) ([]DriveFileMetadata, error) {
	return c.ListDocsByFolder(ctx, "")
}

// ListDocsByFolder lista documentos opcionalmente filtrados por una carpeta específica de Drive
func (c *Client) ListDocsByFolder(ctx context.Context, folderID string) ([]DriveFileMetadata, error) {
	query := "trashed = false and (" +
		"mimeType = 'application/vnd.google-apps.document' or " +
		"mimeType = 'application/vnd.google-apps.spreadsheet' or " +
		"mimeType = 'application/vnd.openxmlformats-officedocument.wordprocessingml.document' or " +
		"mimeType = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' or " +
		"mimeType = 'application/vnd.ms-excel')"

	folderID = strings.TrimSpace(folderID)
	if folderID != "" && folderID != "all" {
		escapedFolder := strings.ReplaceAll(folderID, "'", "\\'")
		query += fmt.Sprintf(" and '%s' in parents", escapedFolder)
	}

	res, err := c.drive.Files.List().
		Q(query).
		Fields("files(id, name, mimeType, parents, modifiedTime)").
		OrderBy("modifiedTime desc").
		PageSize(200).
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("error listando archivos de Drive: %w", err)
	}

	docsList := make([]DriveFileMetadata, 0, len(res.Files))
	for _, f := range res.Files {
		modTime, _ := time.Parse(time.RFC3339, f.ModifiedTime)
		parents := f.Parents
		if parents == nil {
			parents = []string{}
		}
		docsList = append(docsList, DriveFileMetadata{
			ID:        f.Id,
			Name:      f.Name,
			Title:     f.Name,
			MimeType:  f.MimeType,
			Parents:   parents,
			UpdatedAt: modTime,
		})
	}

	return docsList, nil
}

func (c *Client) CreateFolder(ctx context.Context, name string, parentID string) (string, error) {
	folderMetadata := &drive.File{
		Name:     name,
		MimeType: "application/vnd.google-apps.folder",
	}

	if strings.TrimSpace(parentID) != "" && parentID != "root" {
		folderMetadata.Parents = []string{strings.TrimSpace(parentID)}
	}

	res, err := c.drive.Files.Create(folderMetadata).Fields("id, name, parents").Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("error creando carpeta en Drive: %w", err)
	}

	return res.Id, nil
}

// CreateBlankFile crea un Google Doc o Google Sheet nativo en blanco dentro de la carpeta indicada
func (c *Client) CreateBlankFile(ctx context.Context, title string, docType string, folderID string) (*DriveFileMetadata, error) {
	targetMimeType := "application/vnd.google-apps.document"
	normalizedType := strings.ToLower(strings.TrimSpace(docType))
	if normalizedType == "sheet" || normalizedType == "spreadsheet" || normalizedType == "xlsx" || normalizedType == "sheets" {
		targetMimeType = "application/vnd.google-apps.spreadsheet"
	}

	fileMeta := &drive.File{
		Name:     strings.TrimSpace(title),
		MimeType: targetMimeType,
	}
	if strings.TrimSpace(folderID) != "" && folderID != "root" {
		fileMeta.Parents = []string{strings.TrimSpace(folderID)}
	}

	res, err := c.drive.Files.Create(fileMeta).
		Context(ctx).
		Fields("id, name, mimeType, parents, modifiedTime").
		Do()
	if err != nil {
		return nil, fmt.Errorf("error creando documento en blanco en Drive: %w", err)
	}

	modTime, _ := time.Parse(time.RFC3339, res.ModifiedTime)
	if modTime.IsZero() {
		modTime = time.Now()
	}
	parents := res.Parents
	if parents == nil {
		parents = []string{}
	}

	return &DriveFileMetadata{
		ID:        res.Id,
		Name:      res.Name,
		Title:     res.Name,
		MimeType:  res.MimeType,
		Parents:   parents,
		UpdatedAt: modTime,
	}, nil
}

func getClient(config *oauth2.Config, tokFile string) (*http.Client, error) {
	var tok *oauth2.Token
	var err error

	// 1. Soporte prioritario para variable de entorno GOOGLE_TOKEN_JSON (entorno Railway)
	if envTokJSON := strings.TrimSpace(os.Getenv("GOOGLE_TOKEN_JSON")); envTokJSON != "" {
		tok = &oauth2.Token{}
		if decodeErr := json.Unmarshal([]byte(envTokJSON), tok); decodeErr != nil {
			return nil, fmt.Errorf("error parseando GOOGLE_TOKEN_JSON: %w", decodeErr)
		}
	} else {
		// 2. Fallback a archivo local token.json
		tok, err = tokenFromFile(tokFile)
		if err != nil {
			tok, err = getTokenFromWeb(config)
			if err != nil {
				return nil, err
			}
			saveToken(tokFile, tok)
		}
	}

	customTransport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dialer := &net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}
			return dialer.DialContext(ctx, "tcp4", addr)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{
		Transport: customTransport,
		Timeout:   120 * time.Second,
	})

	return config.Client(ctx, tok), nil
}

func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir puerto local: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	config.RedirectURL = fmt.Sprintf("http://localhost:%d", port)

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Printf("\n=======================================================\n")
	fmt.Printf("AUTORIZACIÓN DE GOOGLE DRIVE REQUERIDA (SOLO UNA VEZ)\n")
	fmt.Printf("Abre este enlace en tu navegador con tu cuenta de Gmail:\n\n%v\n", authURL)
	fmt.Printf("=======================================================\n\n")

	codeChan := make(chan string)
	errChan := make(chan error)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			code := r.URL.Query().Get("code")
			if code == "" {
				http.Error(w, "Código no encontrado", http.StatusBadRequest)
				errChan <- fmt.Errorf("no se recibió el código de autorización")
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, "<h2>¡Autorización exitosa!</h2><p>Ya puedes cerrar esta ventana y regresar a tu terminal.</p>")
			codeChan <- code
		}),
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	var code string
	select {
	case code = <-codeChan:
	case err := <-errChan:
		return nil, err
	}

	_ = server.Shutdown(context.Background())

	tok, err := config.Exchange(context.TODO(), code)
	if err != nil {
		return nil, fmt.Errorf("error al intercambiar código por token: %w", err)
	}
	return tok, nil
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Token guardado exitosamente en: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		fmt.Printf("Advertencia: No se pudo guardar el token: %v\n", err)
		return
	}
	defer f.Close()
	_ = json.NewEncoder(f).Encode(token)
}

func NewClient(credentialsPath string) (*Client, error) {
	ctx := context.Background()

	var b []byte
	var err error

	// 1. Soporte prioritario para GOOGLE_CREDENTIALS_JSON (entorno Railway)
	if envCreds := strings.TrimSpace(os.Getenv("GOOGLE_CREDENTIALS_JSON")); envCreds != "" {
		b = []byte(envCreds)
	} else {
		b, err = os.ReadFile(credentialsPath)
		if err != nil {
			return nil, fmt.Errorf("error leyendo credentials.json: %w", err)
		}
	}

	config, err := google.ConfigFromJSON(b, drive.DriveScope, docs.DocumentsScope, sheets.SpreadsheetsScope)
	if err != nil {
		return nil, fmt.Errorf("error parseando credenciales OAuth: %w", err)
	}

	tokenPath := strings.TrimSpace(os.Getenv("GOOGLE_TOKEN_PATH"))
	if tokenPath == "" {
		tokenPath = "token.json"
	}

	httpClient, err := getClient(config, tokenPath)
	if err != nil {
		return nil, fmt.Errorf("error obteniendo cliente HTTP autenticado: %w", err)
	}

	drv, err := drive.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("error al conectar con Google Drive API: %w", err)
	}

	docSvc, err := docs.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("error al conectar con Google Docs API: %w", err)
	}

	sheetsSvc, err := sheets.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("error al conectar con Google Sheets API: %w", err)
	}

	return &Client{drive: drv, docs: docSvc, sheets: sheetsSvc}, nil
}

func (c *Client) UploadDoc(ctx context.Context, fileHeader *multipart.FileHeader) (string, error) {
	meta, err := c.UploadDocInFolder(ctx, fileHeader, "")
	if err != nil {
		return "", err
	}
	return meta.ID, nil
}

func (c *Client) UploadDocInFolder(ctx context.Context, fileHeader *multipart.FileHeader, folderID string) (*DriveFileMetadata, error) {
	src, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	// Detectar si es Excel o Word para convertir al formato nativo correcto
	targetMimeType := "application/vnd.google-apps.document"
	filenameLower := strings.ToLower(fileHeader.Filename)

	if strings.HasSuffix(filenameLower, ".xlsx") || strings.HasSuffix(filenameLower, ".xls") || strings.HasSuffix(filenameLower, ".csv") {
		targetMimeType = "application/vnd.google-apps.spreadsheet"
	}

	fileMetadata := &drive.File{
		Name:     fileHeader.Filename,
		MimeType: targetMimeType,
	}
	if strings.TrimSpace(folderID) != "" && folderID != "root" {
		fileMetadata.Parents = []string{strings.TrimSpace(folderID)}
	}

	res, err := c.drive.Files.Create(fileMetadata).
		Context(ctx).
		Media(src).
		Fields("id, name, mimeType, parents, modifiedTime").
		Do()
	if err != nil {
		return nil, fmt.Errorf("error subiendo archivo a Drive: %w", err)
	}

	modTime, _ := time.Parse(time.RFC3339, res.ModifiedTime)
	if modTime.IsZero() {
		modTime = time.Now()
	}
	parents := res.Parents
	if parents == nil {
		parents = []string{}
	}

	return &DriveFileMetadata{
		ID:        res.Id,
		Name:      res.Name,
		Title:     res.Name,
		MimeType:  res.MimeType,
		Parents:   parents,
		UpdatedAt: modTime,
	}, nil
}

func (c *Client) ExportPDFStream(ctx context.Context, fileID string) (io.ReadCloser, error) {
	res, err := c.drive.Files.Export(fileID, "application/pdf").Context(ctx).Download()
	if err != nil {
		return nil, fmt.Errorf("error exportando a PDF: %w", err)
	}
	return res.Body, nil
}

func (c *Client) ExtractText(ctx context.Context, fileID string) (string, error) {
	doc, err := c.docs.Documents.Get(fileID).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("error obteniendo texto de Docs: %w", err)
	}

	var buf bytes.Buffer
	for _, elem := range doc.Body.Content {
		if elem.Paragraph != nil {
			for _, pElem := range elem.Paragraph.Elements {
				if pElem.TextRun != nil {
					buf.WriteString(pElem.TextRun.Content)
				}
			}
		}
	}

	return buf.String(), nil
}

func (c *Client) MoveFile(ctx context.Context, fileID string, targetFolderID string) error {
	// 1. Obtener los padres actuales del archivo
	file, err := c.drive.Files.Get(fileID).Fields("parents").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error obteniendo carpetas padre del archivo: %w", err)
	}

	previousParents := strings.Join(file.Parents, ",")

	// Si targetFolderID está vacío, lo movemos a 'root'
	if targetFolderID == "" {
		targetFolderID = "root"
	}

	// 2. Ejecutar el update removiendo los padres anteriores y agregando el nuevo
	_, err = c.drive.Files.Update(fileID, nil).
		AddParents(targetFolderID).
		RemoveParents(previousParents).
		Fields("id, parents").
		Context(ctx).
		Do()

	if err != nil {
		return fmt.Errorf("error moviendo archivo en Drive: %w", err)
	}

	return nil
}

func (c *Client) ListFolders(ctx context.Context) ([]DriveFileMetadata, error) {
	q := "mimeType = 'application/vnd.google-apps.folder' and trashed = false"
	res, err := c.drive.Files.List().
		Q(q).
		Fields("files(id, name, mimeType, parents, modifiedTime)").
		OrderBy("name asc").
		PageSize(200).
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("error listando carpetas: %w", err)
	}

	folders := make([]DriveFileMetadata, 0, len(res.Files))
	for _, f := range res.Files {
		modTime, _ := time.Parse(time.RFC3339, f.ModifiedTime)
		parents := f.Parents
		if parents == nil {
			parents = []string{}
		}
		folders = append(folders, DriveFileMetadata{
			ID:        f.Id,
			Name:      f.Name,
			Title:     f.Name,
			MimeType:  f.MimeType,
			Parents:   parents,
			UpdatedAt: modTime,
		})
	}
	return folders, nil
}

func (c *Client) ExportDocumentStream(ctx context.Context, fileID string, mimeType string) (io.ReadCloser, error) {
	res, err := c.drive.Files.Export(fileID, mimeType).Context(ctx).Download()
	if err != nil {
		return nil, fmt.Errorf("error exportando documento: %w", err)
	}
	return res.Body, nil
}

func (c *Client) UpdateText(ctx context.Context, fileID, newContent string) error {
	doc, err := c.docs.Documents.Get(fileID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error obteniendo documento para actualización: %w", err)
	}

	var endIndex int64 = 1
	if len(doc.Body.Content) > 0 {
		lastElem := doc.Body.Content[len(doc.Body.Content)-1]
		endIndex = lastElem.EndIndex - 1
	}

	requests := []*docs.Request{}

	if endIndex > 1 {
		requests = append(requests, &docs.Request{
			DeleteContentRange: &docs.DeleteContentRangeRequest{
				Range: &docs.Range{StartIndex: 1, EndIndex: endIndex},
			},
		})
	}

	if newContent != "" {
		requests = append(requests, &docs.Request{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{Index: 1},
				Text:     newContent,
			},
		})
	}

	if len(requests) == 0 {
		return nil
	}

	_, err = c.docs.Documents.BatchUpdate(fileID, &docs.BatchUpdateDocumentRequest{Requests: requests}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("error aplicando cambios en Docs: %w", err)
	}

	return nil
}
