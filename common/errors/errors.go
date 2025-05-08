package errors

import (
	"github.com/labstack/echo/v4"
	"net/http"
)

type ErrorType string

const (
	ErrorTypeNotFound              ErrorType = "NotFound"
	ErrorTypeServerError           ErrorType = "ServerError"
	ErrorTypeDBError               ErrorType = "DBError"
	ErrorTypeConflict              ErrorType = "Conflict"
	ErrorTypeBadRequest            ErrorType = "BadRequest"
	ErrorTypeMandatory             ErrorType = "Mandatory"
	ErrorTypeUnknown               ErrorType = "Unknown"
	ErrorTypeConfig                ErrorType = "ConfigurationError"
	MaxLimitExceeded               ErrorType = "MaxLimitExceeded"
	ErrorTypeUnauthorized          ErrorType = "Unauthorized"
	ErrorTypeRequestEntityTooLarge ErrorType = "RequestEntityTooLarge"
)

type CommonHedgeError struct {
	errorType ErrorType
	message   string
}

type HedgeError interface {
	ErrorType() ErrorType
	Message() string
	IsErrorType(errorType ErrorType) bool
	Error() string
	ConvertToHTTPError() *echo.HTTPError
}

func (h CommonHedgeError) ErrorType() ErrorType {
	return h.errorType
}

func (h CommonHedgeError) Message() string {
	return h.message
}

func (h CommonHedgeError) Error() string {
	return h.message
}

func (h CommonHedgeError) IsErrorType(errorType ErrorType) bool {
	return errorType == h.errorType
}

func (h CommonHedgeError) ConvertToHTTPError() *echo.HTTPError {
	return echo.NewHTTPError(errorTypeToCode(h.ErrorType()), h.Message())
}

func NewCommonHedgeError(errorType ErrorType, message string) CommonHedgeError {
	return CommonHedgeError{errorType, message}
}

func errorTypeToCode(status ErrorType) int {
	switch status {
	case ErrorTypeServerError:
		return http.StatusInternalServerError
	case ErrorTypeNotFound:
		return http.StatusNotFound
	case ErrorTypeConflict:
		return http.StatusConflict
	case ErrorTypeBadRequest:
		return http.StatusBadRequest
	case ErrorTypeDBError, ErrorTypeUnknown, MaxLimitExceeded:
		return http.StatusInternalServerError
	case ErrorTypeRequestEntityTooLarge:
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusInternalServerError
	}
}
