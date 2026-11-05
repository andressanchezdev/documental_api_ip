package gdrive

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"google.golang.org/api/drive/v3"
)

// UploadedImageInfo contiene los datos de la imagen subida
type UploadedImageInfo struct {
	FileID    string `json:"file_id"`
	FileName  string `json:"file_name"`
	MimeType  string `json:"mime_type"`
	URL       string `json:"url"`        // URL directa CDN para Google Docs e Insertar en Sheets
	ViewerURL string `json:"viewer_url"` // URL de visualización en Google Drive
}

// UploadPublicImage sube una imagen a Google Drive y le otorga permisos públicos
// de lectura ("anyone" con rol "reader") para que la API de Docs y Sheets pueda consumirla.
func (c *Client) UploadPublicImage(ctx context.Context, fileHeader *multipart.FileHeader) (*UploadedImageInfo, error) {
	src, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("error abriendo archivo de imagen: %w", err)
	}
	defer src.Close()

	// Detectar tipo MIME
	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
		switch ext {
		case ".png":
			mimeType = "image/png"
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".gif":
			mimeType = "image/gif"
		case ".webp":
			mimeType = "image/webp"
		case ".svg":
			mimeType = "image/svg+xml"
		case ".bmp":
			mimeType = "image/bmp"
		default:
			// Buffer para sniff mime
			buf := make([]byte, 512)
			n, _ := src.Read(buf)
			mimeType = http.DetectContentType(buf[:n])
			_, _ = src.Seek(0, 0)
		}
	}

	// 1. Crear el archivo en Drive
	fileMetadata := &drive.File{
		Name:     fileHeader.Filename,
		MimeType: mimeType,
	}

	res, err := c.drive.Files.Create(fileMetadata).
		Context(ctx).
		Media(src).
		Fields("id, name, mimeType, webViewLink, webContentLink").
		Do()
	if err != nil {
		return nil, fmt.Errorf("error subiendo imagen a Google Drive: %w", err)
	}

	// 2. Establecer permiso de lectura pública para que la API de Google Docs y Sheets puedan descargarla
	perm := &drive.Permission{
		Role: "reader",
		Type: "anyone",
	}
	_, err = c.drive.Permissions.Create(res.Id, perm).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("error configurando permisos públicos de lectura para la imagen: %w", err)
	}

	// 3. Generar enlace directo de Google
	// La URL lh3.googleusercontent.com/d/{id} es el CDN directo de Google Drive para imágenes públicas
	directURL := fmt.Sprintf("https://lh3.googleusercontent.com/d/%s", res.Id)

	return &UploadedImageInfo{
		FileID:    res.Id,
		FileName:  res.Name,
		MimeType:  res.MimeType,
		URL:       directURL,
		ViewerURL: res.WebViewLink,
	}, nil
}

