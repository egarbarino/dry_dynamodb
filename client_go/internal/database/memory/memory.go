package memory

import "github.com/egarbarino/dry_dynamodb/client_go/internal/model"

// MemorySession is blah
type MemorySession struct {
	users []model.User
}

// NewMemorySession does blah
func NewMemorySession() *MemorySession {
	return &MemorySession{
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

// ListUsers does blah
func (memorySession *MemorySession) ListUsers(lastEvaluatedKey string) ([]model.User, string, error) {
	return memorySession.users, "", nil
}

// LoginUser does blah
func (memorySession *MemorySession) LoginUser(email string) (string, error) {
	for _, v := range memorySession.users {
		if v.Email == email {
			return v.ID, nil
		}
	}
	return "", &model.CustomError{
		ErrorCode:   model.ErrorNoMatch,
		ErrorDetail: email,
	}
}

/*
const (
	// MaxResults is the maximun number of results to display per page
	MaxResults = 7
)
// DbSession is ...type DbSession struct { DynamoDBresource *dynamodb.DynamoDB
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
func (dbSession *DbSession) ListUsers(lastEvaluatedKey string) ([]model.User, string, error) {

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
func (dbSession *DbSession) LoginUser(email string) (string, error) {
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
*/
