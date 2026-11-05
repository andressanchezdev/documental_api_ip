package document

import "time"

type Document struct {
	ID        uint      `json:"id"`
	Title     string    `json:"title"`
	DriveID   string    `json:"drive_id"`
	CreatedBy uint      `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UpdateContentInput struct {
	Content string `json:"content"`
}

type DocumentDetailResponse struct {
	ID        uint      `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	UpdatedAt time.Time `json:"updated_at"`
}