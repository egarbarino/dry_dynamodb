package model

import "fmt"

// User is an user...
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// List is blah blah
type List struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	UserID string `json:"user_id"`
}

// ErrorCode is used for the dbError... enumeration
type ErrorCode int

// dbError Enumeration
const (
	ErrorNoMatch ErrorCode = iota
	ErrorInvalidCount
	ErrorMissingAttribute
	ErrorMarshallingIssue
	ErrorUnimplemented
	ErrorDuplicateID
)

func (e CustomError) Error() string {
	description := "Unknown error"
	switch e.ErrorCode {
	case ErrorNoMatch:
		description = "ErrorNoMatch: Item not found using provided lookup criteria"
	case ErrorInvalidCount:
		description = "ErrorInvalidCount: Constraint violated"
	case ErrorMissingAttribute:
		description = "ErrorMissingAttribute: "
	case ErrorUnimplemented:
		description = "ErrorUnimplemented: not implemented yet"
	case ErrorDuplicateID:
		description = "ErrorDuplicateID: "
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
	ListUsers(lastUserID string, max int64) ([]User, string, error)
	GetUserByEmail(email string) (User, error)
	GetListByListID(listID string) (List, error)
	GetListsByUserID(userID string) ([]List, error)
	CreateList(userID string, title string) (string, error)
	DeleteList(listID string, userID string) error
}
