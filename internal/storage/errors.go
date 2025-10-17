package storage

import "errors"

var (
	ErrUserExist    = errors.New("User already exists")
	ErrUserNotFound = errors.New("User not found")
	ErrAppNotFound  = errors.New("App not found")
	ErrNotFound     = errors.New("not found")
)
