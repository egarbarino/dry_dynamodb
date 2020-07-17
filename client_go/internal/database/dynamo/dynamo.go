package dynamo

import (
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/egarbarino/dry_dynamodb/client_go/internal/model"
	"github.com/google/uuid"
)

// DBSession is a type
type DBSession struct {
	DynamoDBresource *dynamodb.DynamoDB
	slowdownSeconds  int
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

func logConsumedCapacity(method string, c *dynamodb.ConsumedCapacity) {
	if c != nil {
		log.Printf("%s Consumed Capacity (table=%s) = %.2f", method, *c.TableName, *c.CapacityUnits)
	}
}

func logEnd(method string, start time.Time) {
	end := time.Now().Sub(start)
	log.Printf("%s Time (%dms)", method, end.Milliseconds())
}

func slowdown(session *DBSession, method string, description string) {
	if session.slowdownSeconds > 0 {
		log.Printf("%s (%s) Sleeping for %d seconds", method, description, session.slowdownSeconds)
		time.Sleep(time.Duration(session.slowdownSeconds) * time.Second)
	}
}

// Slowdown is a method
func (session *DBSession) Slowdown(seconds int) {
	session.slowdownSeconds = seconds
}

// ListUsers is a method
func (session *DBSession) ListUsers(lastUserID string, max int64) ([]model.User, string, error) {
	const method = "ListUsers"
	slowdown(session, method, "entry")
	defer logEnd(method, time.Now())
	log.Printf("%s (lastUserId=%s,max=%d)", method, lastUserID, max)
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
		TableName:              aws.String("users"),
		Limit:                  aws.Int64(max),
		ExclusiveStartKey:      exclusiveStartKey,
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}
	output, err := session.DynamoDBresource.Scan(input)
	if err != nil {
		return nil, "", err
	}
	logConsumedCapacity(method, output.ConsumedCapacity)
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

// GetUsersByIDs is a method
func (session *DBSession) GetUsersByIDs(ids []string) ([]model.User, error) {
	const method = "GetUsersByIDs"
	slowdown(session, method, "entry")
	defer logEnd(method, time.Now())
	log.Printf("%s (ids=%v)", method, ids)
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
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}

	output, err := session.DynamoDBresource.BatchGetItem(input)
	if err != nil {
		return []model.User{}, err
	}
	for i, v := range output.ConsumedCapacity {
		logConsumedCapacity(fmt.Sprintf("%s #%d", method, i), v)
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

// GetUserByEmail is a method
func (session *DBSession) GetUserByEmail(email string) (model.User, error) {
	const method = "GetUserByEmail"
	slowdown(session, method, "entry")
	defer logEnd(method, time.Now())
	log.Printf("%s (email=%s)", method, email)
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
	logConsumedCapacity(method, output.ConsumedCapacity)
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

// GetAggregateListsByUserID is a method
func (session *DBSession) GetAggregateListsByUserID(userID string) ([]model.AggregateList, error) {

	const method = "GetAggregateListsByUserID"
	slowdown(session, method, "entry")
	defer logEnd(method, time.Now())
	log.Printf("%s (userID=%s)", method, userID)

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

// GetListsByUserID is a method
func (session *DBSession) GetListsByUserID(userID string) ([]model.List, error) {
	const method = "GetListsByUserID"
	slowdown(session, method, "entry")
	defer logEnd(method, time.Now())
	log.Printf("%s (userID=%s)", method, userID)
	dynamoDB := session.DynamoDBresource

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

	output, err := dynamoDB.Query(input)
	if err != nil {
		return []model.List{}, err
	}

	logConsumedCapacity(method, output.ConsumedCapacity)

	var lists = make([]model.List, *output.Count)

	for i, v := range output.Items {
		if err3 := dynamodbattribute.UnmarshalMap(v, &lists[i]); err3 != nil {
			return []model.List{}, err3
		}
	}
	return lists, nil
}

// CreateList is a method
func (session *DBSession) CreateList(userID string, title string) (string, error) {

	const method = "CreateList"
	slowdown(session, method, "entry")
	defer logEnd(method, time.Now())
	log.Printf("%s (userID=%s,title=%s)", method, userID, title)

	uuidString := uuid.New().String()

	user := model.List{
		ID:     uuidString,
		Title:  title,
		UserID: userID,
	}
	userAV, err := dynamodbattribute.MarshalMap(user)
	if err != nil {
		return "", err
	}

	input := &dynamodb.TransactWriteItemsInput{
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
		TransactItems: []*dynamodb.TransactWriteItem{
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
					TableName:           aws.String("lists"),
					Item:                userAV,
					ConditionExpression: aws.String("attribute_not_exists(id)"),
				},
			},
		},
	}

	output, err2 := session.DynamoDBresource.TransactWriteItems(input)
	if err2 != nil {
		switch v := err2.(type) {
		case *dynamodb.TransactionCanceledException:
			switch {
			case len(v.CancellationReasons) > 0 && *v.CancellationReasons[0].Code == "ConditionalCheckFailed":
				return "", &model.CustomError{
					ErrorCode:   model.ErrorNoMatch,
					ErrorDetail: fmt.Sprintf("userID=%s", userID),
				}
			case len(v.CancellationReasons) > 1 && *v.CancellationReasons[1].Code == "ConditionalCheckFailed":
				return "", &model.CustomError{
					ErrorCode:   model.ErrorDuplicateID,
					ErrorDetail: fmt.Sprintf("listID=%s", uuidString),
				}
			default:
				return "", err2
			}
		default:
			return "", err2
		}
	}
	for i, v := range output.ConsumedCapacity {
		logConsumedCapacity(fmt.Sprintf("%s #%d", method, i), v)
	}
	return uuidString, nil
}

// DeleteList is a method
func (session *DBSession) DeleteList(listID string, userID string) error {
	const method = "DeleteList"
	slowdown(session, method, "entry")
	defer logEnd(method, time.Now())
	log.Printf("%s (listID=%s,userID=%s)", method, listID, userID)
	//
	// First mark the lists table as being deleted by adding
	// the `under_deletion` attribute
	//
	log.Printf("Preparing list %s for deletion", listID)
	slowdown(session, method, "before setting under_deletion")
	input := &dynamodb.TransactWriteItemsInput{
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
		TransactItems: []*dynamodb.TransactWriteItem{
			{
				Update: &dynamodb.Update{
					TableName: aws.String("lists"),
					Key: map[string]*dynamodb.AttributeValue{
						"id": {
							S: aws.String(listID),
						},
					},
					UpdateExpression:    aws.String("SET under_deletion = :t"),
					ConditionExpression: aws.String("attribute_exists(id) AND user_id = :u"),
					ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
						":u": {
							S: aws.String(userID),
						},
						":t": {
							BOOL: aws.Bool(true),
						},
					},
				},
			},
		},
	}
	output, err := session.DynamoDBresource.TransactWriteItems(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return &model.CustomError{
					ErrorCode:   model.ErrorNoMatch,
					ErrorDetail: fmt.Sprintf("listID=%s,userID=%s", listID, userID),
				}
			}
			return err
		}
	}
	for i, v := range output.ConsumedCapacity {
		logConsumedCapacity(fmt.Sprintf("%s #%d", method, i), v)
	}
	//
	// Then proceed to delete to obtain all items in the list
	// to then delete them
	//
	items, err := session.GetItemsByListID(listID)
	if err != nil {
		return err
	}
	if len(items) > 0 {
		slowdown(session, method, "before deleting items")
		deleteWriteRequests := make([]*dynamodb.WriteRequest, 0)

		for _, item := range items {
			log.Printf("Deleting item %s (%s)", item.Datetime, item.Description)
			deleteWriteRequests = append(deleteWriteRequests,
				&dynamodb.WriteRequest{
					DeleteRequest: &dynamodb.DeleteRequest{
						Key: map[string]*dynamodb.AttributeValue{
							"list_id": {
								S: aws.String(item.ListID),
							},
							"datetime": {
								S: aws.String(item.Datetime),
							},
						},
					},
				},
			)
		}

		input2 := &dynamodb.BatchWriteItemInput{
			ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
			RequestItems: map[string][]*dynamodb.WriteRequest{
				"items": deleteWriteRequests,
			},
		}

		output2, err2 := session.DynamoDBresource.BatchWriteItem(input2)
		if err2 != nil {
			return err2
		}
		for i, v := range output2.ConsumedCapacity {
			logConsumedCapacity(fmt.Sprintf("%s #%d", method, i), v)
		}
	}

	//
	// Finally, delete the list
	//
	log.Printf("Deleting list %s", listID)
	slowdown(session, method, "before deleting list")
	input3 := &dynamodb.DeleteItemInput{
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
		TableName:              aws.String("lists"),
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

	output3, err3 := session.DynamoDBresource.DeleteItem(input3)
	if err3 != nil {
		if aeer, ok := err3.(awserr.Error); ok {
			if aeer.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return &model.CustomError{
					ErrorCode:   model.ErrorNoMatch,
					ErrorDetail: fmt.Sprintf("listID=%s,userID=%s", listID, userID),
				}
			}
		}
		return err3
	}
	logConsumedCapacity(method, output3.ConsumedCapacity)
	return nil
}

// GetListByListID is blah
func (session *DBSession) GetListByListID(listID string) (model.List, error) {
	const method = "GetListByListID"
	slowdown(session, method, "entry")
	defer logEnd(method, time.Now())
	log.Printf("%s (listID=%s)", method, listID)
	input := &dynamodb.GetItemInput{
		TableName: aws.String("lists"),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(listID),
			},
		},
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}
	output, err := session.DynamoDBresource.GetItem(input)
	if err != nil {
		return model.List{}, err
	}
	logConsumedCapacity(method, output.ConsumedCapacity)
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

// GetListsByIDs is a method
func (session *DBSession) GetListsByIDs(ids []string) ([]model.List, error) {
	const method = "GetListsByIDs"
	slowdown(session, method, "entry")
	defer logEnd(method, time.Now())
	log.Printf("%s (ids=%v)", method, ids)
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
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}
	output, err := session.DynamoDBresource.BatchGetItem(input)
	if err != nil {
		return []model.List{}, err
	}
	for i, v := range output.ConsumedCapacity {
		logConsumedCapacity(fmt.Sprintf("%s #%d", method, i), v)
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

// GetAggregateGuestsByListID is a method
func (session *DBSession) GetAggregateGuestsByListID(listID string) ([]model.AggregateGuest, error) {
	const method = "GetAggregateGuestsByListID"
	slowdown(session, method, "entry")
	defer logEnd(method, time.Now())
	log.Printf("%s (listID=%s)", method, listID)
	guests, err := session.GetGuestsByListID(listID)
	if err != nil {
		return []model.AggregateGuest{}, err
	}
	if len(guests) > 0 {
		var userIDs = make([]string, len(guests))
		for i, v := range guests {
			userIDs[i] = v.UserID
		}
		users, err2 := session.GetUsersByIDs(userIDs)
		if err2 != nil {
			return []model.AggregateGuest{}, err2
		}
		aggregateGuests := make([]model.AggregateGuest, len(guests))
		for ig, vg := range guests {
			aggregateGuests[ig].Guest = vg
			for _, vu := range users {
				if vg.UserID == vu.ID {
					aggregateGuests[ig].Email = vu.Email
				}
			}
		}
		return aggregateGuests, nil
	}
	return []model.AggregateGuest{}, nil
}

// GetGuestsByListID is a method
func (session *DBSession) GetGuestsByListID(listID string) ([]model.Guest, error) {
	const method = "GetGuestsByListID"
	slowdown(session, method, "entry")
	defer logEnd(method, time.Now())
	log.Printf("%s (listID=%v)", method, listID)
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
	logConsumedCapacity(method, output.ConsumedCapacity)

	var guests = make([]model.Guest, *output.Count)

	for i, v := range output.Items {
		if err3 := dynamodbattribute.UnmarshalMap(v, &guests[i]); err3 != nil {
			return []model.Guest{}, err3
		}
	}
	return guests, nil
}

// GetGuestsByUserID is a method
func (session *DBSession) GetGuestsByUserID(userID string) ([]model.Guest, error) {
	const method = "GetGuestsByUserID"
	slowdown(session, method, "entry")
	defer logEnd(method, time.Now())
	log.Printf("%s (userID=%s)", method, userID)
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
	logConsumedCapacity(method, output.ConsumedCapacity)

	var guests = make([]model.Guest, *output.Count)

	for i, v := range output.Items {
		if err3 := dynamodbattribute.UnmarshalMap(v, &guests[i]); err3 != nil {
			return []model.Guest{}, err3
		}
	}
	return guests, nil
}

// IsPresentGuest is a method
func (session *DBSession) IsPresentGuest(listID string, userID string) (bool, error) {
	const method = "IsPresentGuest"
	slowdown(session, method, "entry")
	defer logEnd(method, time.Now())
	log.Printf("%s (listID=%s,userID=%s)", method, listID, userID)
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
	logConsumedCapacity(method, output.ConsumedCapacity)
	if len(output.Item) == 0 {
		return false, nil
	}
	return true, nil
}

// CreateGuest is a method
func (session *DBSession) CreateGuest(listID string, userID string) error {
	const method = "CreateGuest"
	slowdown(session, method, "entry")
	defer logEnd(method, time.Now())
	log.Printf("%s (listID=%s,userID=%s)", method, listID, userID)
	guest := model.Guest{
		ListID: listID,
		UserID: userID,
	}
	guestAV, err := dynamodbattribute.MarshalMap(guest)
	if err != nil {
		return err
	}

	input := &dynamodb.TransactWriteItemsInput{
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
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
					ConditionExpression: aws.String("attribute_not_exists(list_id) AND attribute_not_exists(user_id)"),
				},
			},
		},
	}
	output, err2 := session.DynamoDBresource.TransactWriteItems(input)
	for i, v := range output.ConsumedCapacity {
		logConsumedCapacity(fmt.Sprintf("%s #%d", method, i), v)
	}
	if err2 != nil {
		switch v := err2.(type) {
		case *dynamodb.TransactionCanceledException:
			switch {
			case len(v.CancellationReasons) > 0 && *v.CancellationReasons[0].Code == "ConditionalCheckFailed":
				return &model.CustomError{
					ErrorCode:   model.ErrorNoMatch,
					ErrorDetail: fmt.Sprintf("listID=%s", listID),
				}
			case len(v.CancellationReasons) > 1 && *v.CancellationReasons[1].Code == "ConditionalCheckFailed":
				return &model.CustomError{
					ErrorCode:   model.ErrorNoMatch,
					ErrorDetail: fmt.Sprintf("userID=%s", userID),
				}
			case len(v.CancellationReasons) > 2 && *v.CancellationReasons[2].Code == "ConditionalCheckFailed":
				return &model.CustomError{
					ErrorCode:   model.ErrorDuplicateID,
					ErrorDetail: fmt.Sprintf("listID=%s,userID=%s", listID, userID),
				}
			default:
				return err2
			}
		default:
			return err2
		}
	}
	return nil
}

// DeleteGuest is a method
func (session *DBSession) DeleteGuest(listID string, userID string) error {

	const method = "DeleteGuest"
	slowdown(session, method, "entry")
	defer logEnd(method, time.Now())
	log.Printf("%s (listID=%s,userID=%s)", method, listID, userID)

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
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}

	output, err := session.DynamoDBresource.DeleteItem(input)

	if err != nil {
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
	logConsumedCapacity(method, output.ConsumedCapacity)
	return nil
}

// GetItemsByListID is a method
func (session *DBSession) GetItemsByListID(listID string) ([]model.Item, error) {

	const method = "GetItemsByListID"
	slowdown(session, method, "entry")
	defer logEnd(method, time.Now())
	log.Printf("%s (listID=%s)", method, listID)

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

	logConsumedCapacity(method, output.ConsumedCapacity)

	var items = make([]model.Item, *output.Count)

	for i, v := range output.Items {
		if err3 := dynamodbattribute.UnmarshalMap(v, &items[i]); err3 != nil {
			return []model.Item{}, err3
		}
	}
	return items, nil
}

// CreateItem is a method
func (session *DBSession) CreateItem(listID string, description string) error {

	const method = "CreateItem"
	slowdown(session, method, "entry")
	defer logEnd(method, time.Now())
	log.Printf("%s (listID=%s,description=%s)", method, listID, description)

	datetime := time.Now().Format("2006-01-02T15:04:05.999999")

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
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
		TransactItems: []*dynamodb.TransactWriteItem{
			{
				ConditionCheck: &dynamodb.ConditionCheck{
					TableName: aws.String("lists"),
					Key: map[string]*dynamodb.AttributeValue{
						"id": {
							S: aws.String(listID),
						},
					},
					ConditionExpression: aws.String("attribute_exists(id) AND attribute_not_exists(under_deletion)"),
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

	output, err2 := session.DynamoDBresource.TransactWriteItems(input)
	if err2 != nil {
		switch v := err2.(type) {
		case *dynamodb.TransactionCanceledException:
			switch {
			case len(v.CancellationReasons) > 0 && *v.CancellationReasons[0].Code == "ConditionalCheckFailed":
				return &model.CustomError{
					ErrorCode:   model.ErrorNoMatch,
					ErrorDetail: fmt.Sprintf("listID=%s", listID),
				}
			case len(v.CancellationReasons) > 1 && *v.CancellationReasons[1].Code == "ConditionalCheckFailed":
				return &model.CustomError{
					ErrorCode:   model.ErrorDuplicateID,
					ErrorDetail: fmt.Sprintf("listID=%s,datetime=%s", listID, datetime),
				}
			default:
				return err2
			}
		default:
			return err2
		}
	}
	for i, v := range output.ConsumedCapacity {
		logConsumedCapacity(fmt.Sprintf("%s #%d", method, i), v)
	}
	return nil
}

// DeleteItem is a method
func (session *DBSession) DeleteItem(listID string, datetime string) error {

	const method = "CreateList"
	slowdown(session, method, "entry")
	defer logEnd(method, time.Now())
	log.Printf("%s (listID=%s,listID=%s)", method, listID, datetime)

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
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}
	output, err := session.DynamoDBresource.DeleteItem(input)

	if err != nil {
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
	logConsumedCapacity(method, output.ConsumedCapacity)
	return nil
}

// UpdateItem is a method
func (session *DBSession) UpdateItem(listID string, datetime string, version int, description *string, done *bool) (int, error) {

	const method = "UpdateItem"
	slowdown(session, method, "entry")
	defer logEnd(method, time.Now())
	log.Printf("%s (listID=%s,datetime=%s)", method, listID, datetime)

	updateExpression := "SET version = version + :o"
	expressionAttributeValues := map[string]*dynamodb.AttributeValue{
		":v": {
			N: aws.String(fmt.Sprintf("%d", version)),
		},
		":o": {
			N: aws.String("1"),
		},
	}

	if description != nil {
		updateExpression = updateExpression + ", description = :d"
		expressionAttributeValues[":d"] = &dynamodb.AttributeValue{S: description}
	}
	if done != nil {
		updateExpression = updateExpression + ", done = :n"
		expressionAttributeValues[":n"] = &dynamodb.AttributeValue{BOOL: done}
	}

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String("items"),
		Key: map[string]*dynamodb.AttributeValue{
			"list_id": {
				S: aws.String(listID),
			},
			"datetime": {
				S: aws.String(datetime),
			},
		},
		UpdateExpression:          aws.String(updateExpression),
		ConditionExpression:       aws.String("version = :v"),
		ExpressionAttributeValues: expressionAttributeValues,
		ReturnValues:              aws.String(dynamodb.ReturnValueUpdatedNew),
		ReturnConsumedCapacity:    aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}

	output, err := session.DynamoDBresource.UpdateItem(input)
	if err != nil {
		if aeer, ok := err.(awserr.Error); ok {
			if aeer.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return 0, &model.CustomError{
					ErrorCode:   model.ErrorNoMatch,
					ErrorDetail: fmt.Sprintf("listID=%s,datetime=%s,version=%d", listID, datetime, version),
				}
			}
		}
		return 0, err
	}
	logConsumedCapacity(method, output.ConsumedCapacity)
	v := output.Attributes["version"]
	if v == nil {
		return 0, &model.CustomError{
			ErrorCode:   model.ErrorMissingAttribute,
			ErrorDetail: "version",
		}
	}
	newVersion, err2 := strconv.Atoi(*v.N)
	if err2 != nil {
		return 0, err2
	}

	return newVersion, nil
}
