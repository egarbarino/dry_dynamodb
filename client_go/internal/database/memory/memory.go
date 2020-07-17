package memory

import (
	"fmt"

	"github.com/egarbarino/dry_dynamodb/client_go/internal/model"
)

// Session is a type
type Session struct {
	users []model.User
}

// New initialises a dummy data set
func New() *Session {
	return &Session{
		users: []model.User{
			{
				ID:    "7c2be6b9-746c-44be-bb33-78fb402ce6b8",
				Email: "gwalker@hotmail.com",
			},
			{
				ID:    "a10f9a38-f6dc-4e8a-ac1c-180486389697",
				Email: "wdean@gmail.com",
			},
			{
				ID:    "d5fc9ce9-5a5d-4ffc-9cc1-20a5c865bcc7",
				Email: "millsshawn@henry.com",
			},
		},
	}
}

// ListUsers is a method
func (memorySession *Session) ListUsers(lastUserID string, max int64) ([]model.User, string, error) {
	var users = make([]model.User, 0)
	var counter int64 = 0
	collecting := false
	lastIndex := 0
	lastSeenUserID := ""

	if lastUserID == "" {
		collecting = true
	}
	for i, v := range memorySession.users {
		if collecting {
			users = append(users, v)
			lastSeenUserID = v.ID
			lastIndex = i
			counter++
			if counter >= (max) {
				break
			}
		}
		if v.ID == lastUserID {
			collecting = true
		}
	}
	if lastIndex == len(memorySession.users)-1 {
		lastSeenUserID = ""
	}
	return users, lastSeenUserID, nil
}

// Slowdown is a method
func (memorySession *Session) Slowdown(seconds int) {

}

// GetUsersByIDs is a method
func (memorySession *Session) GetUsersByIDs(ids []string) ([]model.User, error) {
	users := make([]model.User, 1)
	for _, u := range memorySession.users {
		for _, id := range ids {
			if u.ID == id {
				users = append(users, u)
			}
		}
	}
	if len(users) > 0 {
		return users, nil
	}
	return []model.User{}, &model.CustomError{
		ErrorCode:   model.ErrorNoMatch,
		ErrorDetail: fmt.Sprintf("%v", ids),
	}
}

// GetUserByEmail is a method
func (memorySession *Session) GetUserByEmail(email string) (model.User, error) {
	for _, v := range memorySession.users {
		if v.Email == email {
			return v, nil
		}
	}
	return model.User{}, &model.CustomError{
		ErrorCode:   model.ErrorNoMatch,
		ErrorDetail: email,
	}
}

// GetAggregateListsByUserID is a method
func (memorySession *Session) GetAggregateListsByUserID(lastListID string) ([]model.AggregateList, error) {
	return []model.AggregateList{}, &model.CustomError{
		ErrorCode:   model.ErrorUnimplemented,
		ErrorDetail: "Interface.GetAggregateListsByUserID",
	}
}

// GetListsByUserID is a method
func (memorySession *Session) GetListsByUserID(lastListID string) ([]model.List, error) {
	return []model.List{}, &model.CustomError{
		ErrorCode:   model.ErrorUnimplemented,
		ErrorDetail: "Interface.GetListsByUserID",
	}
}

// CreateList is a method
func (memorySession *Session) CreateList(userID string, title string) (string, error) {
	return "", &model.CustomError{
		ErrorCode:   model.ErrorUnimplemented,
		ErrorDetail: "Interface.CreateList",
	}
}

// DeleteList is a method
func (memorySession *Session) DeleteList(listID string, userID string) error {
	return &model.CustomError{
		ErrorCode:   model.ErrorUnimplemented,
		ErrorDetail: "Interface.DeleteList",
	}
}

// GetListByListID is a method
func (memorySession *Session) GetListByListID(listID string) (model.List, error) {
	return model.List{}, &model.CustomError{
		ErrorCode:   model.ErrorUnimplemented,
		ErrorDetail: "Interface.GetListByListID",
	}
}

// GetAggregateGuestsByListID is a method
func (memorySession *Session) GetAggregateGuestsByListID(listID string) ([]model.AggregateGuest, error) {
	return []model.AggregateGuest{}, &model.CustomError{
		ErrorCode:   model.ErrorUnimplemented,
		ErrorDetail: "Interface.GetAggregateGuestsByUserID",
	}
}

// GetGuestsByListID is a method
func (memorySession *Session) GetGuestsByListID(listID string) ([]model.Guest, error) {
	return []model.Guest{}, &model.CustomError{
		ErrorCode:   model.ErrorUnimplemented,
		ErrorDetail: "Interface.GetGuestsByListID",
	}
}

// GetGuestsByUserID is a method
func (memorySession *Session) GetGuestsByUserID(userID string) ([]model.Guest, error) {
	return []model.Guest{}, &model.CustomError{
		ErrorCode:   model.ErrorUnimplemented,
		ErrorDetail: "Interface.GetGuestsByUserID",
	}
}

// CreateGuest is a method
func (memorySession *Session) CreateGuest(listID string, userID string) error {
	return &model.CustomError{
		ErrorCode:   model.ErrorUnimplemented,
		ErrorDetail: "Interface.CreateGuest",
	}
}

// DeleteGuest is a method
func (memorySession *Session) DeleteGuest(listID string, userID string) error {
	return &model.CustomError{
		ErrorCode:   model.ErrorUnimplemented,
		ErrorDetail: "Interface.DeleteGuest",
	}
}

// IsPresentGuest is a method
func (memorySession *Session) IsPresentGuest(listID string, userID string) (bool, error) {
	return false, &model.CustomError{
		ErrorCode:   model.ErrorUnimplemented,
		ErrorDetail: "Interface.IsPresentGuest",
	}
}

// GetItemsByListID is a method
func (memorySession *Session) GetItemsByListID(listID string) ([]model.Item, error) {
	return []model.Item{}, &model.CustomError{
		ErrorCode:   model.ErrorUnimplemented,
		ErrorDetail: "Interface.GetItemsByListID",
	}
}

// CreateItem is a method
func (memorySession *Session) CreateItem(listID string, description string) error {
	return &model.CustomError{
		ErrorCode:   model.ErrorUnimplemented,
		ErrorDetail: "Interface.CreateItem",
	}
}

// DeleteItem is a method
func (memorySession *Session) DeleteItem(listID string, datetime string) error {
	return &model.CustomError{
		ErrorCode:   model.ErrorUnimplemented,
		ErrorDetail: "Interface.DeleteItem",
	}
}

// UpdateItem is a method
func (memorySession *Session) UpdateItem(listID string, datetime string, version int, description *string, done *bool) (int, error) {
	return 0, &model.CustomError{
		ErrorCode:   model.ErrorUnimplemented,
		ErrorDetail: "Interface.UpdateItem",
	}
}
