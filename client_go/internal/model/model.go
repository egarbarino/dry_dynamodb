package model

import "fmt"

// User is a type
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// List is a type
type List struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	UserID string `json:"user_id"`
}

// AggregateList is a type
type AggregateList struct {
	List
	GuestCount int
	ItemCount  int
	AsGuest    bool
}

// Guest is a type
type Guest struct {
	ListID string `json:"list_id"`
	UserID string `json:"user_id"`
}

// AggregateGuest is a type
type AggregateGuest struct {
	Guest
	Email string
}

// Item is a type
type Item struct {
	ListID      string `json:"list_id"`
	Datetime    string `json:"datetime"`
	Description string `json:"description"`
	Done        bool   `json:"done"`
	Order       int    `json:"order"`
	Version     int    `json:"version"`
}

// dbError Enumeration
const (
	ErrorNoMatch ErrorCode = iota
	ErrorInvalidCount
	ErrorMissingAttribute
	ErrorMarshallingIssue
	ErrorUnimplemented
	ErrorDuplicateID
)

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

// Interface is what it says on the tin
type Interface interface {
	Slowdown(seconds int)
	ListUsers(lastUserID string, max int64) ([]User, string, error)
	GetUsersByIDs(ids []string) ([]User, error)
	GetUserByEmail(email string) (User, error)
	GetListByListID(listID string) (List, error)
	GetAggregateListsByUserID(userID string) ([]AggregateList, error)
	GetListsByUserID(userID string) ([]List, error)
	CreateList(userID string, title string) (string, error)
	DeleteList(listID string, userID string) error
	GetAggregateGuestsByListID(listID string) ([]AggregateGuest, error)
	GetGuestsByListID(listID string) ([]Guest, error)
	GetGuestsByUserID(userID string) ([]Guest, error)
	CreateGuest(listID string, userID string) error
	DeleteGuest(listID string, userID string) error
	IsPresentGuest(listID string, userID string) (bool, error)
	GetItemsByListID(listID string) ([]Item, error)
	CreateItem(listID string, description string) error
	DeleteItem(listID string, datetime string) error
	UpdateItem(listID string, datetime string, version int, description *string, done *bool) (int, error)
}
