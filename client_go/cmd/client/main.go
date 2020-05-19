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

const maxResults = 2

// UserSession does blah blah
type UserSession struct {
	backend          model.Interface
	loggedUser       model.User
	selectedList     model.List
	lastEvaluatedKey string
	lastCommand      string
}

func help() {
	fmt.Print("" +
		"General Options:\n" +
		"   help                    Show options\n" +
		"   users                   List users\n" +
		"   login user@domain.com   Log in with user account\n" +
		"   exit                    Exit application\n" +
		"For Logged-in Users\n" +
		"   lists                   Show User's To Do lists\n" +
		"   list create NAME        Create a new list\n" +
		"   list delete ListID      Delete existing list\n")
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
			users, lastEvaluatedKey, err := session.backend.ListUsers(session.lastEvaluatedKey, maxResults)
			if err != nil {
				fmt.Println(err)
				break
			}
			session.lastEvaluatedKey = lastEvaluatedKey

			if len(users) > 0 {
				fmt.Printf("%-37s  %-50s\n", "User Id", "Email")
				fmt.Printf("%-37s  %-50s\n", "-------", "-----")
				for _, user := range users {
					fmt.Printf("%-37s  %-50s\n", user.ID, user.Email)
				}
				if lastEvaluatedKey != "" && len(users) == maxResults {
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
				break
			}
			email := text[len("login "):]
			fmt.Printf("email: %s\n", email)
			user, err := session.backend.GetUserByEmail(email) // wgamble@fields.com
			if err != nil {
				fmt.Println(err)
				break
			}
			session.loggedUser = user
			fmt.Println("Succesful login")
			break

		// Lists
		case strings.HasPrefix(text, "lists"):
			if session.loggedUser.Email == "" {
				fmt.Println("This command requires a logged user via the login command")
				break
			}
			lists, err := session.backend.GetListsByUserID(session.loggedUser.ID)
			if err != nil {
				fmt.Println(err)
				break
			}

			if len(lists) > 0 {
				fmt.Printf("%-37s  %-50s\n", "List Id", "Title")
				fmt.Printf("%-37s  %-50s\n", "-------", "-----")
				for _, list := range lists {
					fmt.Printf("%-37s  %-50s\n", list.ID, list.Title)
				}
			}

		// List Select
		case strings.HasPrefix(text, "list select"):
			if session.loggedUser.ID == "" {
				fmt.Println("This command requires a logged user via the login command")
				break
			}
			if len(text) < len("list select _") {
				fmt.Println("No list specified")
				break
			}
			listID := text[len("list select "):]
			list, err := session.backend.GetListByListID(listID)
			if err != nil {
				fmt.Println(err)
				break
			}
			if list.UserID != session.loggedUser.ID {
				fmt.Println("This list does not belong to you!")
				break
			}
			session.selectedList = list

		// List Create
		case strings.HasPrefix(text, "list create"):
			if session.loggedUser.ID == "" {
				fmt.Println("This command requires a logged user via the login command")
				break
			}
			if len(text) < len("list create _") {
				fmt.Println("No title specified")
				break
			}
			title := text[len("list create "):]
			listID, err := session.backend.CreateList(session.loggedUser.ID, title)
			if err != nil {
				fmt.Println(err)
				break
			}
			fmt.Printf("List %s created\n", listID)

		// List Delete
		case strings.HasPrefix(text, "list delete"):
			if session.loggedUser.ID == "" {
				fmt.Println("This command requires a logged user via the login command")
				break
			}
			if len(text) < len("list delete _") {
				fmt.Println("No list Id specified")
				break
			}
			listID := text[len("list delete "):]
			if err := session.backend.DeleteList(listID, session.loggedUser.ID); err != nil {
				fmt.Println(err)
				break
			}
			fmt.Printf("List %s deleted\n", listID)

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

	fmt.Print("*** Todo List Application ***\n")
	fmt.Print("Usage: ./client memory | ./client (default using DynamoDB)\n")
	if len(os.Args) > 1 && os.Args[1] == "memory" {
		fmt.Print("\nMemory backend selected\n\n")
		backend = memory.New()
	} else {
		fmt.Print("\nDynamoDB backend selected\n\n")
		session := session.Must(session.NewSessionWithOptions(session.Options{
			Profile: "dynamodb_profile",
			Config: aws.Config{
				Region: aws.String("eu-west-2"),
			},
		}))
		backend = &dynamo.DBSession{DynamoDBresource: dynamodb.New(session)}
	}

	help()

	inputLoop(&UserSession{backend: backend})

}
