package apperrors

import "fmt"

// ErrCRLNotFound возвращается, когда CRL не найден.
var ErrCRLNotFound = fmt.Errorf("crl not found")

// Newf создаёт новую ошибку с форматированием.
func Newf(code, message string, httpStatus int, args ...any) error {
	return fmt.Errorf("%s: %s (httpStatus=%d)", code, fmt.Sprintf(message, args...), httpStatus)
}

// Wrap оборачивает ошибку с дополнительным контекстом.
func Wrap(err error, code, message string, httpStatus int) error {
	return fmt.Errorf("%s: %s (httpStatus=%d): %w", code, message, httpStatus, err)
}
