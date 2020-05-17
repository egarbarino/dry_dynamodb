package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const (
	// MaxResults is the maximun number of results to display per page
	MaxResults = 7
)

// User is an user...
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// DbErrorCode is used for the dbError... enumeration
type DbErrorCode int

// dbError Enumeration
const (
	DbErrorNoMatch DbErrorCode = iota
	DbErrorInvalidCount
	DbErrorMissingAttribute
	DbErrorMarshallingIssue
)

// DbError is a custom error
type DbError struct {
	ErrorCode   DbErrorCode
	ErrorDetail string
}

// DbSession is ...
type DbSession struct {
	dynamoDBresource *dynamodb.DynamoDB
}

// UserSession does blah blah
type UserSession struct {
	dbSession        *DbSession
	loggedUser       User
	lastEvaluatedKey string
	lastCommand      string
}

func (e DbError) Error() string {
	description := "Unknown error"
	switch e.ErrorCode {
	case DbErrorNoMatch:
		description = "DbErrorNoMatch: Item not found using provided look up criteria"
	case DbErrorInvalidCount:
		description = "DbErrorInvalidCount: Constraint violated"
	case DbErrorMissingAttribute:
		description = "DbErrorMissingAttribute: "
	}
	return fmt.Sprintf("%s (%s)", description, e.ErrorDetail)
}

func validateQueryOutputCount(count int64, queryOutput *dynamodb.QueryOutput) error {
	if count != -1 {
		if *queryOutput.Count != count {
			return &DbError{
				ErrorCode:   DbErrorInvalidCount,
				ErrorDetail: fmt.Sprintf("intended=%d, actual=%d", count, *queryOutput.Count),
			}
		}
	}
	return nil
}

func dbListUsers(dbSession *DbSession, lastEvaluatedKey string) ([]User, string, error) {

	svc := dbSession.dynamoDBresource
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
		var users = make([]User, *scanOutput.Count)
		for index, scanItem := range scanOutput.Items {
			err2 := dynamodbattribute.UnmarshalMap(scanItem, &users[index])
			if err2 != nil {
				return nil, "", err2
			}
		}
		return users, lastEvaluatedKey, nil
	}
	return []User{}, lastEvaluatedKey, nil
}

func Login(dbSession *DbSession, email string) (string, error) {
	svc := dbSession.dynamoDBresource
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
		return "", &DbError{
			ErrorCode:   DbErrorNoMatch,
			ErrorDetail: email,
		}
	}
	if err2 := validateQueryOutputCount(1, queryOutput); err2 != nil {
		return "", err2
	}

	userIDAttribute, present := queryOutput.Items[0]["id"]
	if !present {
		return "", &DbError{
			ErrorCode:   DbErrorMissingAttribute,
			ErrorDetail: "id",
		}
	}

	userID := *userIDAttribute.S
	return userID, nil

}

func help() {
	fmt.Print("" +
		"Options:\n" +
		"   help                    Show options\n" +
		"   users                   List users\n" +
		"   login user@domain.com   Log in with user account\n" +
		"   exit                    Exit application\n")
}

func inputLoop(session *UserSession) {

	var scanner *bufio.Scanner
	var text string

	for {
		fmt.Printf("%s> ", session.loggedUser.Email)
		scanner = bufio.NewScanner(os.Stdin)
		if err := scanner.Err(); err != nil {
			log.Fatal(err)
			break
		}
		if !scanner.Scan() {
			log.Fatal("No lines")
			break
		}
		text = scanner.Text()
		switch {
		// Help
		case strings.HasPrefix(text, "help") || text == "":
			help()

		// Users
		case strings.HasPrefix(text, "users") || (session.lastCommand == "users" && text == "n"):
			if strings.HasPrefix(text, "users") {
				session.lastEvaluatedKey = ""
			}
			users, lastEvaluatedKey, err := dbListUsers(session.dbSession, session.lastEvaluatedKey)
			if err != nil {
				fmt.Println(err)
				break
			}
			session.lastEvaluatedKey = lastEvaluatedKey

			if len(users) > 0 {
				fmt.Printf("%-40s  %-50s\n", "User Id", "Email")
				for _, user := range users {
					fmt.Printf("%-40s  %-50s\n", user.ID, user.Email)
				}
				if lastEvaluatedKey != "" && len(users) == MaxResults {
					session.lastCommand = "users"
					fmt.Println("Type 'n' to see more results")
				} else {
					fmt.Println("--- End of list ---")
				}
			} else {
				fmt.Println("No further results. Type 'n' again to start from the beginning.")
			}

		// Login
		case strings.HasPrefix(text, "login"):
			if len(text) < len("login _") {
				fmt.Println("No email specified")
			}
			email := text[len("login "):]
			fmt.Printf("email: %s\n", email)
			result, err := dbLogin(session.dbSession, email) // wgamble@fields.com
			if err != nil {
				fmt.Println(err)
				break
			}
			session.loggedUser = User{
				ID:    result,
				Email: email,
			}
			fmt.Println("Succesful login")
			break

		// Exit
		case strings.HasPrefix(text, "exit"):
			return

		// Next without context
		case text == "n":
			fmt.Println("There is no context for the 'n' (next) command")

		// Unknown command
		default:
			fmt.Printf("%s is not a valid command.\n", text)
			break

		}
	}
}

func main() {

	session := session.Must(session.NewSessionWithOptions(session.Options{
		Profile: "dynamodb_profile",
		Config: aws.Config{
			Region: aws.String("eu-west-2"),
		},
	}))
	fmt.Print("*** Todo List Application ***\n\n")
	help()

	inputLoop(&UserSession{
		dbSession: &DbSession{
			dynamodb.New(session),
		},
	})

}

/*
	result, err := svc.Query(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeProvisionedThroughputExceededException:
				fmt.Println(dynamodb.ErrCodeProvisionedThroughputExceededException, aerr.Error())
			case dynamodb.ErrCodeResourceNotFoundException:
				fmt.Println(dynamodb.ErrCodeResourceNotFoundException, aerr.Error())
			case dynamodb.ErrCodeRequestLimitExceeded:
				fmt.Println(dynamodb.ErrCodeRequestLimitExceeded, aerr.Error())
			case dynamodb.ErrCodeInternalServerError:
				fmt.Println(dynamodb.ErrCodeInternalServerError, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return "", "blah" // fix

*/
