package model

import "fmt"

// dbError Enumeration
const (
	ErrorNoMatch ErrorCode = iota
	ErrorInvalidCount
	ErrorMissingAttribute
	ErrorMarshallingIssue
	ErrorUnimplemented
	ErrorDuplicateID
)

// User is an user...
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// List is blah blah
type List struct {
	ID                  string `json:"id"`
	Title               string `json:"title"`
	UserID              string `json:"user_id"`
	AggregateGuestCount int
}

// AggregateList is blah blah
type AggregateList struct {
	List
	AggregateGuestCount int
}

// Guest is blah
type Guest struct {
	ListID string `json:"list_id"`
	UserID string `json:"user_id"`
	// Field not populated directly from Table
	AggregateEmail string
}

// ErrorCode is used for the dbError... enumeration
type ErrorCode int

// CustomError is a custom error
type CustomError struct {
	ErrorCode   ErrorCode
	ErrorDetail string
}

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

// Interface is magic
type Interface interface {
	ListUsers(lastUserID string, max int64) ([]User, string, error)
	GetUsersByIDs(ids []string) ([]User, error)
	GetUserByEmail(email string) (User, error)
	GetListByListID(listID string) (List, error)
	GetListsByUserID(userID string) ([]List, error)
	CreateList(userID string, title string) (string, error)
	DeleteList(listID string, userID string) error
	GetAggregateGuestsByListID(listID string) ([]Guest, error)
	GetGuestsByListID(listID string) ([]Guest, error)
}
