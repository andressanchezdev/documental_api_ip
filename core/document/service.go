package document

import (
	"context"
	"io"
	"mime/multipart"

	"doc-api/core/platform/gdrive"
)

type Service interface {
	Upload(ctx context.Context, file *multipart.FileHeader, userID uint) (*Document, error)
	List() ([]*Document, error)
	GetPreview(ctx context.Context, docID uint) (io.ReadCloser, string, error)
	GetContent(ctx context.Context, docID uint) (*DocumentDetailResponse, error)
	UpdateContent(ctx context.Context, docID uint, newContent string) error
}

type service struct {
	repo   Repository
	gdrive *gdrive.Client
}

func NewService(repo Repository, gdrive *gdrive.Client) Service {
	return &service{repo: repo, gdrive: gdrive}
}

func (s *service) Upload(ctx context.Context, file *multipart.FileHeader, userID uint) (*Document, error) {
	driveID, err := s.gdrive.UploadDoc(ctx, file)
	if err != nil {
		return nil, err
	}
	return s.repo.Create(file.Filename, driveID, userID)
}

func (s *service) List() ([]*Document, error) {
	return s.repo.FindAll()
}

func (s *service) GetPreview(ctx context.Context, docID uint) (io.ReadCloser, string, error) {
	doc, err := s.repo.FindByID(docID)
	if err != nil {
		return nil, "", err
	}

	stream, err := s.gdrive.ExportPDFStream(ctx, doc.DriveID)
	if err != nil {
		return nil, "", err
	}

	return stream, doc.Title, nil
}

func (s *service) GetContent(ctx context.Context, docID uint) (*DocumentDetailResponse, error) {
	doc, err := s.repo.FindByID(docID)
	if err != nil {
		return nil, err
	}

	content, err := s.gdrive.ExtractText(ctx, doc.DriveID)
	if err != nil {
		return nil, err
	}

	return &DocumentDetailResponse{
		ID:        doc.ID,
		Title:     doc.Title,
		Content:   content,
		UpdatedAt: doc.UpdatedAt,
	}, nil
}

func (s *service) UpdateContent(ctx context.Context, docID uint, newContent string) error {
	doc, err := s.repo.FindByID(docID)
	if err != nil {
		return err
	}

	if err := s.gdrive.UpdateText(ctx, doc.DriveID, newContent); err != nil {
		return err
	}

	return s.repo.Touch(doc.ID)
}
