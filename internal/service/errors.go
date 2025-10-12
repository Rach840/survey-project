package service

import "errors"

var (
	PasswordIncorrect = errors.New("password Incorrect")
	TokenIncorrect    = errors.New("token Incorrect")
)
