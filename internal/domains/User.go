package domains

import (
	"time"
)

type User struct {
	Id         int        `json:"id"`
	FullName   string     `json:"full_name" `
	Email      string     `json:"email" `
	Role       string     `json:"role"`
	CreatedAt  time.Time  `json:"created_at"`
	DisabledAt *time.Time `sql:"disabled_at,omitempty" json:"disabled_at"`
}
