package providers

import "github.com/jackc/pgx/v5/pgxpool"

type Providers struct {
	AuthProvider     *AuthProvider
	SurveyProvider   *SurveyProvider
	TemplateProvider *TemplateProvider
	UserProvider     *UserProvider
}

func New(db *pgxpool.Pool) *Providers {
	authProvider := NewAuthProvider(db)
	surveyProvider := NewSurveyProvider(db)
	templateProvider := NewTemplateProvider(db)
	userProvider := NewUserProvider(db)
	return &Providers{
		AuthProvider:     authProvider,
		SurveyProvider:   surveyProvider,
		TemplateProvider: templateProvider,
		UserProvider:     userProvider,
	}
}
