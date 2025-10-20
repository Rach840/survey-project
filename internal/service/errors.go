package service

import "errors"

var (
	PasswordIncorrect            = errors.New("password Incorrect")
	TokenIncorrect               = errors.New("token Incorrect")
	ErrSurveyTokenInvalid        = errors.New("survey token invalid")
	ErrSurveyTokenExpired        = errors.New("survey token expired")
	ErrSurveyTokenUsed           = errors.New("survey token usage limit reached")
	ErrSurveyResponseNotFound    = errors.New("survey response not found")
	ErrSurveyParticipantExists   = errors.New("survey participant already exists")
	ErrSurveyParticipantNotFound = errors.New("survey participant not found")
)
