package providers

import "github.com/jackc/pgx/v5/pgxpool"

type Providers struct {
	AuthProvider   *AuthProvider
	SurveyProvider *SurveyProvider
}

func New(db *pgxpool.Pool) *Providers {
	authProvider := NewAuthProvider(db)
	surveyProvider := NewSurveyProvider(db)

	return &Providers{
		AuthProvider:   authProvider,
		SurveyProvider: surveyProvider,
	}
}
