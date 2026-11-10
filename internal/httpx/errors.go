package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/jalilbengoufa/go-api-template/internal/domain"
)

const StatusClientClosedRequest = 499

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

type APIError struct {
	Status  int
	Code    string
	Message string
	Fields  map[string]string
	err     error
}

func (e *APIError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return e.Message
}

func (e *APIError) Unwrap() error { return e.err }

func NewError(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

func BadRequest(message string) *APIError {
	return NewError(http.StatusBadRequest, "bad_request", message)
}

func NotFound(message string) *APIError {
	return NewError(http.StatusNotFound, "not_found", message)
}

func Conflict(message string) *APIError {
	return NewError(http.StatusConflict, "conflict", message)
}

func Unauthorized(message string) *APIError {
	return NewError(http.StatusUnauthorized, "unauthorized", message)
}

func Forbidden(message string) *APIError {
	return NewError(http.StatusForbidden, "forbidden", message)
}

func Internal(err error) *APIError {
	return &APIError{
		Status:  http.StatusInternalServerError,
		Code:    "internal_error",
		Message: "an unexpected error occurred",
		err:     err,
	}
}

func ValidationFailed(fields map[string]string) *APIError {
	return &APIError{
		Status:  http.StatusUnprocessableEntity,
		Code:    "validation_failed",
		Message: "request validation failed",
		Fields:  fields,
	}
}

func ErrorHandler(logger *slog.Logger) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}

		apiErr := toAPIError(err)

		if apiErr.Status >= http.StatusInternalServerError {
			logger.ErrorContext(c.Request().Context(), "request failed",
				slog.String("method", c.Request().Method),
				slog.String("path", c.Request().URL.Path),
				slog.Int("status", apiErr.Status),
				slog.Any("error", err),
			)
		}

		body := ErrorResponse{Error: ErrorBody{
			Code:      apiErr.Code,
			Message:   apiErr.Message,
			Fields:    apiErr.Fields,
			RequestID: RequestID(c),
		}}

		if c.Request().Method == http.MethodHead {
			err = c.NoContent(apiErr.Status)
		} else {
			err = c.JSON(apiErr.Status, body)
		}
		if err != nil {
			logger.ErrorContext(c.Request().Context(), "failed to write error response", slog.Any("error", err))
		}
	}
}

func toAPIError(err error) *APIError {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return NewError(http.StatusGatewayTimeout, "request_timeout", "the request took too long to process")
	case errors.Is(err, context.Canceled):
		return NewError(StatusClientClosedRequest, "client_closed_request", "the client closed the connection")
	case errors.Is(err, domain.ErrNotFound):
		return NotFound(err.Error())
	case errors.Is(err, domain.ErrConflict):
		return Conflict(err.Error())
	case errors.Is(err, domain.ErrInvalidInput):
		return BadRequest(err.Error())
	case errors.Is(err, domain.ErrUnauthorized):
		return Unauthorized(err.Error())
	case errors.Is(err, domain.ErrForbidden):
		return Forbidden(err.Error())
	}

	var echoErr *echo.HTTPError
	if errors.As(err, &echoErr) {
		message := http.StatusText(echoErr.Code)
		if m, ok := echoErr.Message.(string); ok && m != "" {
			message = m
		}
		return NewError(echoErr.Code, codeForStatus(echoErr.Code), message)
	}

	return Internal(err)
}

func codeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusConflict:
		return "conflict"
	case http.StatusRequestEntityTooLarge:
		return "payload_too_large"
	case http.StatusUnsupportedMediaType:
		return "unsupported_media_type"
	case http.StatusUnprocessableEntity:
		return "validation_failed"
	case http.StatusTooManyRequests:
		return "rate_limit_exceeded"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	case http.StatusGatewayTimeout:
		return "request_timeout"
	case StatusClientClosedRequest:
		return "client_closed_request"
	default:
		if status >= http.StatusInternalServerError {
			return "internal_error"
		}
		return "request_error"
	}
}
