package model

import "fmt"

// User is an user...
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// ErrorCode is used for the dbError... enumeration
type ErrorCode int

// dbError Enumeration
const (
	ErrorNoMatch ErrorCode = iota
	ErrorInvalidCount
	ErrorMissingAttribute
	ErrorMarshallingIssue
)

func (e CustomError) Error() string {
	description := "Unknown error"
	switch e.ErrorCode {
	case ErrorNoMatch:
		description = "ErrorNoMatch: Item not found using provided look up criteria"
	case ErrorInvalidCount:
		description = "ErrorInvalidCount: Constraint violated"
	case ErrorMissingAttribute:
		description = "ErrorMissingAttribute: "
	}
	return fmt.Sprintf("%s (%s)", description, e.ErrorDetail)
}

// CustomError is a custom error
type CustomError struct {
	ErrorCode   ErrorCode
	ErrorDetail string
}

// Interface is magic
type Interface interface {
	ListUsers(lastEvaluatedKey string) ([]User, string, error)
	LoginUser(email string) (string, error)
}
