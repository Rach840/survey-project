package domains

type Questioner struct {
	Id       string `json:"id"`
	FullName string `json:"full_name" `
	Email    string `json:"email" `
	Password string `json:"password"`
	Role     string `json:"role"`
}
