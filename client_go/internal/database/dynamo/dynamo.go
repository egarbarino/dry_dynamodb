package dynamo

import (
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/egarbarino/dry_dynamodb/client_go/internal/model"
	"github.com/google/uuid"
)

// DBSession is ...
type DBSession struct {
	DynamoDBresource *dynamodb.DynamoDB
}

func validateQueryOutputCount(count int64, output *dynamodb.QueryOutput) error {
	if count != -1 {
		if *output.Count != count {
			return &model.CustomError{
				ErrorCode:   model.ErrorInvalidCount,
				ErrorDetail: fmt.Sprintf("intended=%d, actual=%d", count, *output.Count),
			}
		}
	}
	return nil
}

// ListUsers  does blah
func (session *DBSession) ListUsers(lastUserID string, max int64) ([]model.User, string, error) {

	var exclusiveStartKey map[string]*dynamodb.AttributeValue

	if lastUserID != "" {
		exclusiveStartKey = map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(lastUserID),
			},
		}
	} else {
		exclusiveStartKey = nil
	}
	input := &dynamodb.ScanInput{
		TableName:         aws.String("users"),
		Limit:             aws.Int64(max),
		ExclusiveStartKey: exclusiveStartKey,
	}
	output, err := session.DynamoDBresource.Scan(input)
	if err != nil {
		return nil, "", err
	}
	if output.LastEvaluatedKey != nil {
		lastUserID = *output.LastEvaluatedKey["id"].S
	} else {
		lastUserID = ""
	}
	if *output.Count > 0 {
		var users = make([]model.User, *output.Count)
		for index, scanItem := range output.Items {
			err2 := dynamodbattribute.UnmarshalMap(scanItem, &users[index])
			if err2 != nil {
				return nil, "", err2
			}
		}
		return users, lastUserID, nil
	}
	return []model.User{}, lastUserID, nil
}

// GetUsersByIDs does blah
func (session *DBSession) GetUsersByIDs(ids []string) ([]model.User, error) {

	var keys = make([]map[string]*dynamodb.AttributeValue, len(ids))
	for i, v := range ids {
		keys[i] = map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(v),
			},
		}
	}
	input := &dynamodb.BatchGetItemInput{
		RequestItems: map[string]*dynamodb.KeysAndAttributes{
			"users": {
				Keys: keys,
			},
		},
	}

	output, err := session.DynamoDBresource.BatchGetItem(input)
	if err != nil {
		return []model.User{}, err
	}

	usersAttributes := output.Responses["users"]
	if len(usersAttributes) > 0 {
		var users = make([]model.User, len(usersAttributes))
		for i, v := range usersAttributes {
			if err = dynamodbattribute.UnmarshalMap(v, &users[i]); err != nil {
				return []model.User{}, err
			}
		}
		return users, nil
	}
	return []model.User{}, nil
}

// GetUserByEmail ..
func (session *DBSession) GetUserByEmail(email string) (model.User, error) {
	svc := session.DynamoDBresource
	input := &dynamodb.QueryInput{
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":v1": {
				S: aws.String(email),
			},
		},
		KeyConditionExpression: aws.String("email = :v1"),
		ProjectionExpression:   aws.String("id"),
		TableName:              aws.String("users"),
		IndexName:              aws.String("users_by_email"),
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}
	output, err := svc.Query(input)
	if err != nil {
		return model.User{}, err
	}
	if *output.Count == 0 {
		return model.User{}, &model.CustomError{
			ErrorCode:   model.ErrorNoMatch,
			ErrorDetail: email,
		}
	}
	if err2 := validateQueryOutputCount(1, output); err2 != nil {
		return model.User{}, err2
	}

	var user model.User

	if err3 := dynamodbattribute.UnmarshalMap(output.Items[0], &user); err3 != nil {
		return model.User{}, err3
	}

	user.Email = email
	return user, nil

}

// GetAggregateListsByUserID does blah
func (session *DBSession) GetAggregateListsByUserID(userID string) ([]model.AggregateList, error) {

	type listsResult struct {
		result  []model.List
		asGuest bool
		err     error
	}
	type countResult struct {
		listID string
		count  int
		err    error
	}

	var wg sync.WaitGroup

	listsChan := make(chan listsResult, 2)
	guestCountChan := make(chan countResult, 100)
	itemCountChan := make(chan countResult, 100)

	countGuests := func(session *DBSession, wg *sync.WaitGroup, listID string) {
		defer wg.Done()
		guests, err := session.GetGuestsByListID(listID)
		guestCountChan <- countResult{
			listID: listID,
			count:  len(guests),
			err:    err,
		}
	}

	countItems := func(session *DBSession, wg *sync.WaitGroup, listID string) {
		defer wg.Done()
		items, err := session.GetItemsByListID(listID)
		itemCountChan <- countResult{
			listID: listID,
			count:  len(items),
			err:    err,
		}
	}

	// Retrieve the listIDs where the current userID
	// is a guest (they don't own the lists)

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		guests, err := session.GetGuestsByUserID(userID)
		if err != nil {
			listsChan <- listsResult{
				result: []model.List{},
				err:    err,
			}
			return
		}
		listIDs := make([]string, len(guests))
		for i, g := range guests {
			listIDs[i] = g.ListID
			wg.Add(2)
			go countGuests(session, wg, g.ListID)
			go countItems(session, wg, g.ListID)
		}
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			lists, err := session.GetListsByIDs(listIDs)
			listsChan <- listsResult{
				result:  lists,
				asGuest: true,
				err:     err,
			}
		}(wg)
	}(&wg)

	// Retrieve the lists owned by the userID
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		list, err := session.GetListsByUserID(userID)
		listsChan <- listsResult{result: list, err: err}
		for _, l := range list {
			wg.Add(2)
			go countGuests(session, wg, l.ID)
			go countItems(session, wg, l.ID)
		}
	}(&wg)

	wg.Wait()
	close(listsChan)
	close(guestCountChan)
	close(itemCountChan)

	guestCountMap := make(map[string]int)
	for guestCount := range guestCountChan {
		if guestCount.err != nil {
			return []model.AggregateList{}, guestCount.err
		}
		guestCountMap[guestCount.listID] = guestCount.count
	}

	itemCountMap := make(map[string]int)
	for itemCount := range itemCountChan {
		if itemCount.err != nil {
			return []model.AggregateList{}, itemCount.err
		}
		itemCountMap[itemCount.listID] = itemCount.count
	}

	alists := make([]model.AggregateList, 0, 1)
	for r := range listsChan {
		if r.err != nil {
			return []model.AggregateList{}, r.err
		}
		for _, l := range r.result {
			alists = append(alists, model.AggregateList{
				List:       l,
				GuestCount: guestCountMap[l.ID],
				ItemCount:  itemCountMap[l.ID],
				AsGuest:    r.asGuest,
			})
		}
	}

	return alists, nil
}

// GetListsByUserID does blah
func (session *DBSession) GetListsByUserID(userID string) ([]model.List, error) {

	svc := session.DynamoDBresource
	input := &dynamodb.QueryInput{
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":v1": {
				S: aws.String(userID),
			},
		},
		KeyConditionExpression: aws.String("user_id = :v1"),
		ProjectionExpression:   aws.String("id,title"),
		TableName:              aws.String("lists"),
		IndexName:              aws.String("lists_by_user_id"),
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}

	output, err := svc.Query(input)
	if err != nil {
		return []model.List{}, err
	}

	var lists = make([]model.List, *output.Count)

	for i, v := range output.Items {
		if err3 := dynamodbattribute.UnmarshalMap(v, &lists[i]); err3 != nil {
			return []model.List{}, err3
		}
	}
	return lists, nil
}

// CreateList does ..
func (session *DBSession) CreateList(userID string, title string) (string, error) {

	uuidString := uuid.New().String()
	// uuidString = "df7224d7-93aa-4395-bb96-aada170d55e1"

	user := model.List{
		ID:     uuidString,
		Title:  title,
		UserID: userID,
	}
	userAV, err := dynamodbattribute.MarshalMap(user)
	if err != nil {
		return "", err
	}
	input := &dynamodb.PutItemInput{
		TableName:           aws.String("lists"),
		Item:                userAV,
		ConditionExpression: aws.String("attribute_not_exists(id)"),
	}
	_, err2 := session.DynamoDBresource.PutItem(input)
	if err2 != nil {
		if aerr, ok := err2.(awserr.Error); ok {
			if aerr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return "", &model.CustomError{
					ErrorCode:   model.ErrorDuplicateID,
					ErrorDetail: uuidString,
				}
			}
			return "", err2
		}
	}
	return uuidString, nil
}

// DeleteList does blah
func (session *DBSession) DeleteList(listID string, userID string) error {
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String("lists"),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(listID),
			},
		},
		ConditionExpression: aws.String("attribute_exists(id) AND user_id = :u"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":u": {
				S: aws.String(userID),
			},
		},
	}

	if _, err := session.DynamoDBresource.DeleteItem(input); err != nil {
		if aeer, ok := err.(awserr.Error); ok {
			if aeer.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return &model.CustomError{
					ErrorCode:   model.ErrorNoMatch,
					ErrorDetail: fmt.Sprintf("listID=%s,userID=%s", listID, userID),
				}
			}
		}
		return err
	}
	return nil
}

// GetListByListID does blah
func (session *DBSession) GetListByListID(listID string) (model.List, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String("lists"),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(listID),
			},
		},
	}
	output, err := session.DynamoDBresource.GetItem(input)
	if err != nil {
		return model.List{}, err
	}

	var list model.List

	if err2 := dynamodbattribute.UnmarshalMap(output.Item, &list); err2 != nil {
		return model.List{}, err2
	}
	if list.ID == "" {
		return model.List{}, &model.CustomError{
			ErrorCode:   model.ErrorNoMatch,
			ErrorDetail: listID,
		}
	}

	return list, nil
}

// GetListsByIDs does blah TODO! Make public
func (session *DBSession) GetListsByIDs(ids []string) ([]model.List, error) {

	var keys = make([]map[string]*dynamodb.AttributeValue, len(ids))
	for i, v := range ids {
		keys[i] = map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(v),
			},
		}
	}
	input := &dynamodb.BatchGetItemInput{
		RequestItems: map[string]*dynamodb.KeysAndAttributes{
			"lists": {
				Keys: keys,
			},
		},
	}

	output, err := session.DynamoDBresource.BatchGetItem(input)
	if err != nil {
		return []model.List{}, err
	}

	listsAttributes := output.Responses["lists"]
	if len(listsAttributes) > 0 {
		var lists = make([]model.List, len(listsAttributes))
		for i, v := range listsAttributes {
			if err = dynamodbattribute.UnmarshalMap(v, &lists[i]); err != nil {
				return []model.List{}, err
			}
		}
		return lists, nil
	}
	return []model.List{}, nil
}

// GetAggregateGuestsByListID is blah
func (session *DBSession) GetAggregateGuestsByListID(listID string) ([]model.Guest, error) {
	guests, err := session.GetGuestsByListID(listID)
	if err != nil {
		return []model.Guest{}, err
	}
	var userIDs = make([]string, len(guests))
	for i, v := range guests {
		userIDs[i] = v.UserID
	}
	if len(userIDs) > 0 {
		users, err2 := session.GetUsersByIDs(userIDs)
		if err2 != nil {
			return []model.Guest{}, err2
		}
		for ig, vg := range guests {
			for _, vu := range users {
				if vg.UserID == vu.ID {
					guests[ig].AggregateEmail = vu.Email
				}
			}
		}
	}
	// fmt.Println(guests)
	return guests, nil
}

// GetGuestsByListID does blah
func (session *DBSession) GetGuestsByListID(listID string) ([]model.Guest, error) {

	input := &dynamodb.QueryInput{
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":v1": {
				S: aws.String(listID),
			},
		},
		KeyConditionExpression: aws.String("list_id = :v1"),
		TableName:              aws.String("guests"),
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}

	output, err := session.DynamoDBresource.Query(input)
	if err != nil {
		return []model.Guest{}, err
	}

	var guests = make([]model.Guest, *output.Count)

	for i, v := range output.Items {
		if err3 := dynamodbattribute.UnmarshalMap(v, &guests[i]); err3 != nil {
			return []model.Guest{}, err3
		}
	}
	return guests, nil
}

// GetGuestsByUserID does blah
func (session *DBSession) GetGuestsByUserID(userID string) ([]model.Guest, error) {

	input := &dynamodb.QueryInput{
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":v1": {
				S: aws.String(userID),
			},
		},
		KeyConditionExpression: aws.String("user_id = :v1"),
		TableName:              aws.String("guests"),
		IndexName:              aws.String("guests_by_user_id"),
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}
	output, err := session.DynamoDBresource.Query(input)
	if err != nil {
		return []model.Guest{}, err
	}

	var guests = make([]model.Guest, *output.Count)

	for i, v := range output.Items {
		if err3 := dynamodbattribute.UnmarshalMap(v, &guests[i]); err3 != nil {
			return []model.Guest{}, err3
		}
	}
	return guests, nil
}

// IsPresentGuest does blah
func (session *DBSession) IsPresentGuest(listID string, userID string) (bool, error) {

	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"list_id": {
				S: aws.String(listID),
			},
			"user_id": {
				S: aws.String(userID),
			},
		},
		TableName:              aws.String("guests"),
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}
	output, err := session.DynamoDBresource.GetItem(input)
	if err != nil {
		return false, err
	}
	if len(output.Item) == 0 {
		return false, nil
	}
	return true, nil
}

// CreateGuest does ..
func (session *DBSession) CreateGuest(listID string, userID string) error {

	uuidString := uuid.New().String()
	// uuidString = "df7224d7-93aa-4395-bb96-aada170d55e1"

	guest := model.Guest{
		ListID: listID,
		UserID: userID,
	}
	guestAV, err := dynamodbattribute.MarshalMap(guest)
	if err != nil {
		return err
	}
	/*
		input := &dynamodb.PutItemInput{
			TableName:           aws.String("guests"),
			Item:                guestAV,
			ConditionExpression: aws.String("attribute_not_exists(id)"),
		}
	*/
	input2 := &dynamodb.TransactWriteItemsInput{
		TransactItems: []*dynamodb.TransactWriteItem{
			{
				ConditionCheck: &dynamodb.ConditionCheck{

					TableName: aws.String("lists"),
					Key: map[string]*dynamodb.AttributeValue{
						"id": {
							S: aws.String(listID),
						},
					},
					ConditionExpression: aws.String("attribute_exists(id)"),
				},
			},
			{
				ConditionCheck: &dynamodb.ConditionCheck{

					TableName: aws.String("users"),
					Key: map[string]*dynamodb.AttributeValue{
						"id": {
							S: aws.String(userID),
						},
					},
					ConditionExpression: aws.String("attribute_exists(id)"),
				},
			},
			{
				Put: &dynamodb.Put{
					TableName:           aws.String("guests"),
					Item:                guestAV,
					ConditionExpression: aws.String("attribute_not_exists(id)"),
				},
			},
		},
	}
	_, err2 := session.DynamoDBresource.PutItem(input2)
	if err2 != nil {
		if aerr, ok := err2.(awserr.Error); ok {
			if aerr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return &model.CustomError{
					ErrorCode:   model.ErrorDuplicateID,
					ErrorDetail: uuidString,
				}
			}
			return err2
		}
	}
	return nil
}

// DeleteGuest does blah
func (session *DBSession) DeleteGuest(listID string, userID string) error {
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String("guests"),
		Key: map[string]*dynamodb.AttributeValue{
			"list_id": {
				S: aws.String(listID),
			},
			"user_id": {
				S: aws.String(userID),
			},
		},
		ConditionExpression: aws.String("list_id = :l AND user_id = :u"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":l": {
				S: aws.String(listID),
			},
			":u": {
				S: aws.String(userID),
			},
		},
	}

	if _, err := session.DynamoDBresource.DeleteItem(input); err != nil {
		if aeer, ok := err.(awserr.Error); ok {
			if aeer.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return &model.CustomError{
					ErrorCode:   model.ErrorNoMatch,
					ErrorDetail: fmt.Sprintf("listID=%s,userID=%s", listID, userID),
				}
			}
		}
		return err
	}
	return nil
}

// GetItemsByListID does blah
func (session *DBSession) GetItemsByListID(listID string) ([]model.Item, error) {

	input := &dynamodb.QueryInput{
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":list_id": {
				S: aws.String(listID),
			},
		},
		KeyConditionExpression: aws.String("list_id = :list_id"),
		TableName:              aws.String("items"),
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}

	output, err := session.DynamoDBresource.Query(input)
	if err != nil {
		return []model.Item{}, err
	}

	var items = make([]model.Item, *output.Count)

	for i, v := range output.Items {
		if err3 := dynamodbattribute.UnmarshalMap(v, &items[i]); err3 != nil {
			return []model.Item{}, err3
		}
	}
	return items, nil
}

// CreateItem does ..
func (session *DBSession) CreateItem(listID string, description string) error {

	datetime := time.Now().Format("2006-01-02T15:04:05.999999")
	// datetime = "2020-05-31T19:56:41.295828"

	item := model.Item{
		ListID:      listID,
		Datetime:    datetime,
		Description: description,
		Done:        false,
		Order:       10,
	}

	itemAV, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		return err
	}
	input := &dynamodb.TransactWriteItemsInput{
		TransactItems: []*dynamodb.TransactWriteItem{
			{
				ConditionCheck: &dynamodb.ConditionCheck{
					TableName: aws.String("lists"),
					Key: map[string]*dynamodb.AttributeValue{
						"id": {
							S: aws.String(listID),
						},
					},
					ConditionExpression: aws.String("attribute_exists(id)"),
				},
			},
			{
				Put: &dynamodb.Put{
					TableName:           aws.String("items"),
					Item:                itemAV,
					ConditionExpression: aws.String("attribute_not_exists(list_id) AND attribute_not_exists(#d)"),
					ExpressionAttributeNames: map[string]*string{
						"#d": aws.String("datetime"),
					},
				},
			},
		},
	}

	_, err2 := session.DynamoDBresource.TransactWriteItems(input)
	if err2 != nil {
		if aerr, ok := err2.(awserr.Error); ok {
			if aerr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return &model.CustomError{
					ErrorCode:   model.ErrorDuplicateID,
					ErrorDetail: fmt.Sprintf("list_id=%s,datetime=%s", listID, datetime),
				}
			}
			return err2
		}
	}
	return nil
}

// DeleteItem does blah
func (session *DBSession) DeleteItem(listID string, datetime string) error {
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String("items"),
		Key: map[string]*dynamodb.AttributeValue{
			"list_id": {
				S: aws.String(listID),
			},
			"datetime": {
				S: aws.String(datetime),
			},
		},
		ConditionExpression: aws.String("list_id = :l AND #d = :u"),
		ExpressionAttributeNames: map[string]*string{
			"#d": aws.String("datetime"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":l": {
				S: aws.String(listID),
			},
			":u": {
				S: aws.String(datetime),
			},
		},
	}
	if _, err := session.DynamoDBresource.DeleteItem(input); err != nil {
		if aeer, ok := err.(awserr.Error); ok {
			if aeer.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return &model.CustomError{
					ErrorCode:   model.ErrorNoMatch,
					ErrorDetail: fmt.Sprintf("listID=%s,datetime=%s", listID, datetime),
				}
			}
		}
		return err
	}
	return nil
}
