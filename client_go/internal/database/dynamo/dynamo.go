package dynamo

import (
	"fmt"

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
	svc := session.DynamoDBresource
	input := &dynamodb.PutItemInput{
		TableName:           aws.String("lists"),
		Item:                userAV,
		ConditionExpression: aws.String("attribute_not_exists(id)"),
	}
	_, err2 := svc.PutItem(input)
	if err2 != nil {
		if aerr, ok := err2.(awserr.Error); ok {
			if aerr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return "", &model.CustomError{
					ErrorCode:   model.ErrorDuplicateID,
					ErrorDetail: uuidString,
				}
			}
			return "", err
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
		fmt.Println(users)
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

// GetGuestsByIDs does blah
func (session *DBSession) GetXXXGuestsByIDs(ids []string) ([]model.User, error) {

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
