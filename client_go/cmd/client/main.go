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
	"github.com/egarbarino/dry_dynamodb/client_go/internal/database/dynamo"
	"github.com/egarbarino/dry_dynamodb/client_go/internal/database/memory"
	"github.com/egarbarino/dry_dynamodb/client_go/internal/model"
)

// UserSession does blah blah
type UserSession struct {
	backend          model.Interface
	loggedUser       model.User
	lastEvaluatedKey string
	lastCommand      string
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
			users, lastEvaluatedKey, err := session.backend.ListUsers(session.lastEvaluatedKey)
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
				if lastEvaluatedKey != "" && len(users) == dynamo.MaxResults {
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
			result, err := session.backend.LoginUser(email) // wgamble@fields.com
			if err != nil {
				fmt.Println(err)
				break
			}
			session.loggedUser = model.User{
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

	var backend model.Interface

	fmt.Print("*** Todo List Application ***\n\n")
	fmt.Print("Choose back-end:\n")
	fmt.Print("   1) Memory\n")
	fmt.Print("   2) DynamoDB\n\n")
	for {
		fmt.Print("> ")
		var scanner *bufio.Scanner
		var text string
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
		if text == "1" {
			backend = memory.NewMemorySession()
			break
		}
		if text == "2" {
			session := session.Must(session.NewSessionWithOptions(session.Options{
				Profile: "dynamodb_profile",
				Config: aws.Config{
					Region: aws.String("eu-west-2"),
				},
			}))
			backend = &dynamo.DynamoDBSession{DynamoDBresource: dynamodb.New(session)}
			break
		}
		fmt.Println("Unknown option")
	}

	help()

	inputLoop(&UserSession{backend: backend})

}
