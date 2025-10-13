package domains

import (
	"encoding/json"
	"time"
)

type TemplateCreate struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     int    `json:"version"`
	Section     string `json:"sections"`
}
type Template struct {
	ID                  int64           `sql:"id" json:"id"`
	OwnerID             int64           `sql:"owner_id" json:"owner_id"`
	Title               string          `sql:"title" json:"title"`
	Description         *string         `sql:"description,omitempty" json:"description,omitempty"`
	Version             int             `sql:"version" json:"version"`
	Status              string          `sql:"status" json:"status"`
	DraftSchemaJSON     json.RawMessage `sql:"draft_schema_json" json:"draft_schema_json"`
	PublishedSchemaJSON json.RawMessage `sql:"published_schema_json,omitempty" json:"published_schema_json,omitempty"`
	UpdatedAt           time.Time       `sql:"updated_at" json:"updated_at"`
	PublishedAt         *time.Time      `sql:"published_at,omitempty" json:"published_at,omitempty"`
}
