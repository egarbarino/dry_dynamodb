package dynamo

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/egarbarino/dry_dynamodb/client_go/internal/model"
)

const (
	// MaxResults is the maximun number of results to display per page
	MaxResults = 7
)

// DynamoDBSession is ...
type DynamoDBSession struct {
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
func (dbSession *DynamoDBSession) ListUsers(lastEvaluatedKey string) ([]model.User, string, error) {

	svc := dbSession.DynamoDBresource
	var scanInput = new(dynamodb.ScanInput)
	if lastEvaluatedKey == "" {
		scanInput = &dynamodb.ScanInput{
			TableName: aws.String("users"),
			Limit:     aws.Int64(MaxResults),
		}
	} else {
		scanInput = &dynamodb.ScanInput{
			TableName: aws.String("users"),
			Limit:     aws.Int64(MaxResults),
			ExclusiveStartKey: map[string]*dynamodb.AttributeValue{
				"id": {
					S: aws.String(lastEvaluatedKey),
				},
			},
		}
	}
	scanOutput, err := svc.Scan(scanInput)
	if err != nil {
		return nil, "", err
	}
	if scanOutput.LastEvaluatedKey != nil {
		lastEvaluatedKey = *scanOutput.LastEvaluatedKey["id"].S
	} else {
		lastEvaluatedKey = ""
	}

	if *scanOutput.Count > 0 {
		var users = make([]model.User, *scanOutput.Count)
		for index, scanItem := range scanOutput.Items {
			err2 := dynamodbattribute.UnmarshalMap(scanItem, &users[index])
			if err2 != nil {
				return nil, "", err2
			}
		}
		return users, lastEvaluatedKey, nil
	}
	return []model.User{}, lastEvaluatedKey, nil
}

// LoginUser does blah ....
func (dbSession *DynamoDBSession) LoginUser(email string) (string, error) {
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
		return "", err
	}
	if *queryOutput.Count == 0 {
		return "", &model.CustomError{
			ErrorCode:   model.ErrorNoMatch,
			ErrorDetail: email,
		}
	}
	if err2 := validateQueryOutputCount(1, queryOutput); err2 != nil {
		return "", err2
	}

	userIDAttribute, present := queryOutput.Items[0]["id"]
	if !present {
		return "", &model.CustomError{
			ErrorCode:   model.ErrorMissingAttribute,
			ErrorDetail: "id",
		}
	}

	userID := *userIDAttribute.S
	return userID, nil

}
