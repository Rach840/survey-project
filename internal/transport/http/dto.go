package httptransport

type LoginData struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type TokenRefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}
