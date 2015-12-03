package http

import "fmt"

// HTTPError implements the error interface but also optionally holds the HTTP
// status code for the appropriate error. If left to 0 the server should send
// 500
type HTTPError struct {
	message    string
	statusCode int
}

func (e HTTPError) Error() string {
	return e.message
}

func (e HTTPError) Code() int {
	return e.statusCode
}

func NewError(statusCode int, msg string, args ...interface{}) HTTPError {
	return HTTPError{
		message:    fmt.Sprintf(msg, args...),
		statusCode: statusCode,
	}
}
