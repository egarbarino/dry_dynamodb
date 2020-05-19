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

func validateQueryOutputCount(count int64, queryOutput *dynamodb.QueryOutput) error {
	if count != -1 {
		if *queryOutput.Count != count {
			return &model.CustomError{
				ErrorCode:   model.ErrorInvalidCount,
				ErrorDetail: fmt.Sprintf("intended=%d, actual=%d", count, *queryOutput.Count),
			}
		}
	}
	return nil
}

// ListUsers  does blah
func (dbSession *DBSession) ListUsers(lastUserID string, max int64) ([]model.User, string, error) {

	svc := dbSession.DynamoDBresource
	var scanInput = new(dynamodb.ScanInput)
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
	scanInput = &dynamodb.ScanInput{
		TableName:         aws.String("users"),
		Limit:             aws.Int64(max),
		ExclusiveStartKey: exclusiveStartKey,
	}
	scanOutput, err := svc.Scan(scanInput)
	if err != nil {
		return nil, "", err
	}
	if scanOutput.LastEvaluatedKey != nil {
		lastUserID = *scanOutput.LastEvaluatedKey["id"].S
	} else {
		lastUserID = ""
	}

	if *scanOutput.Count > 0 {
		var users = make([]model.User, *scanOutput.Count)
		for index, scanItem := range scanOutput.Items {
			err2 := dynamodbattribute.UnmarshalMap(scanItem, &users[index])
			if err2 != nil {
				return nil, "", err2
			}
		}
		return users, lastUserID, nil
	}
	return []model.User{}, lastUserID, nil
}

// GetUserByEmail ..
func (dbSession *DBSession) GetUserByEmail(email string) (model.User, error) {
	svc := dbSession.DynamoDBresource
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
	queryOutput, err := svc.Query(input)
	if err != nil {
		return model.User{}, err
	}
	if *queryOutput.Count == 0 {
		return model.User{}, &model.CustomError{
			ErrorCode:   model.ErrorNoMatch,
			ErrorDetail: email,
		}
	}
	if err2 := validateQueryOutputCount(1, queryOutput); err2 != nil {
		return model.User{}, err2
	}

	var user model.User

	if err3 := dynamodbattribute.UnmarshalMap(queryOutput.Items[0], &user); err3 != nil {
		return model.User{}, err3
	}

	user.Email = email
	return user, nil

}

// GetListsByUserID does blah
func (dbSession *DBSession) GetListsByUserID(userID string) ([]model.List, error) {

	svc := dbSession.DynamoDBresource
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

	queryOutput, err := svc.Query(input)
	if err != nil {
		return []model.List{}, err
	}

	var lists = make([]model.List, *queryOutput.Count)

	for i, v := range queryOutput.Items {
		if err3 := dynamodbattribute.UnmarshalMap(v, &lists[i]); err3 != nil {
			return []model.List{}, err3
		}
	}
	return lists, nil
}

// CreateList does ..
func (dbSession *DBSession) CreateList(userID string, title string) (string, error) {

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
	svc := dbSession.DynamoDBresource
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
func (dbSession *DBSession) DeleteList(listID string, userID string) error {
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

	if _, err := dbSession.DynamoDBresource.DeleteItem(input); err != nil {
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
func (dbSession *DBSession) GetListByListID(listID string) (model.List, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String("lists"),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(listID),
			},
		},
	}
	output, err := dbSession.DynamoDBresource.GetItem(input)
	if err != nil {
		if aeer, ok := err.(awserr.Error); ok {
			if aeer.Code() == dynamodb.ErrCodeResourceNotFoundException {
				return model.List{}, &model.CustomError{
					ErrorCode:   model.ErrorNoMatch,
					ErrorDetail: listID,
				}
			}
		}
	}
	var list model.List

	if err2 := dynamodbattribute.UnmarshalMap(output.Item, &list); err2 != nil {
		return model.List{}, err2
	}

	return list, nil
}
