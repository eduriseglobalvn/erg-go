package exception

import "net/http"

type AppError struct {
	Code       string
	Message    string
	HTTPStatus int
	Details    any
	Cause      error
}

func New(code, message string, status int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: status,
	}
}

func Wrap(code, message string, status int, cause error) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: status,
		Cause:      cause,
	}
}

func (e *AppError) Error() string {
	return e.Code + ": " + e.Message
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

func (e *AppError) Status() int {
	if e == nil || e.HTTPStatus == 0 {
		return http.StatusInternalServerError
	}
	return e.HTTPStatus
}
