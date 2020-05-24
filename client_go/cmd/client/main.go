package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/egarbarino/dry_dynamodb/client_go/internal/database/dynamo"
	"github.com/egarbarino/dry_dynamodb/client_go/internal/database/memory"
	"github.com/egarbarino/dry_dynamodb/client_go/internal/model"
)

const maxResults = 5

// UserSession does blah blah
type UserSession struct {
	backend          model.Interface
	loggedUser       model.User
	selectedList     model.List
	lastEvaluatedKey string
	lastCommand      string
	sequenceList     []string
	sequenceCounter  int
}

func help() {
	fmt.Print("" +
		"General Options:\n" +
		"   help                    Show options\n" +
		"   users                   List users\n" +
		"   user UserID             Select UserID\n" +
		"   email user@domain.com   Select User by Email\n" +
		"   seq                     Reset sequence counter\n" +
		"   exit                    Exit application\n" +
		"Once a user is selected\n" +
		"   lists                   Show User's To Do lists\n" +
		"   list ListID             Select a List\n" +
		"   list create NAME        Create a new list\n" +
		"   list delete ListID      Delete existing list\n" +
		"Once a list is selected\n" +
		"   guests                  List guests invited to the list\n")
}

func inputLoop(session *UserSession) {

	var scanner *bufio.Scanner
	var text string
	session.sequenceCounter = 0
	session.sequenceList = make([]string, 1000)

	for {
		fmt.Printf("%s|%s> ", session.loggedUser.Email, session.selectedList.Title)
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

		// Replace $1, $232, etc with saved data

		r, _ := regexp.Compile("\\$([0-9]+)")
		for _, v := range r.FindAllString(text, 9) {
			if n, err := strconv.Atoi(v[1:]); err != nil {
				fmt.Printf("%s is an invalid sequence.\n", v)
				break
			} else {
				if n >= session.sequenceCounter {
					fmt.Printf("%s hasn't been set yet.\n", v)
					break
				}
				text = strings.ReplaceAll(text, v, session.sequenceList[n])
			}
		}

		switch {
		// Help
		case strings.HasPrefix(text, "help") || text == "":
			help()

		case strings.HasPrefix(text, "seq"):
			session.sequenceCounter = 0

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
				fmt.Printf("%-4s %-37s  %-50s\n", "Seq", "UserID", "Email")
				fmt.Printf("%-4s %-37s  %-50s\n", "---", "------", "-----")
				for _, user := range users {
					fmt.Printf("%03d  %-37s  %-50s\n", session.sequenceCounter, user.ID, user.Email)
					session.sequenceList[session.sequenceCounter] = user.ID
					session.sequenceCounter++
				}
				fmt.Println("---")
				fmt.Printf("Use Seq numbers in lieu of IDs. For example, 'user $%d'\n", session.sequenceCounter-1)
				if lastEvaluatedKey != "" && len(users) == maxResults {
					session.lastCommand = "users"
					fmt.Println("Type 'n' to see more results")
				} else {
					fmt.Println("--- End of list ---")
				}
			} else {
				fmt.Println("No further results. Type 'n' again to start from the beginning.")
			}

		// Select User by Email
		case strings.HasPrefix(text, "email"):
			if len(text) < len("email _") {
				fmt.Println("No email specified")
				break
			}
			email := text[len("email "):]
			user, err := session.backend.GetUserByEmail(email) // wgamble@fields.com
			if err != nil {
				fmt.Println(err)
				break
			}
			session.loggedUser = user
			session.selectedList = model.List{}
			break

		// Select User by ID
		case strings.HasPrefix(text, "user"):
			if len(text) < len("user _") {
				fmt.Println("No ID specified")
				break
			}
			userID := text[len("user "):]
			users, err := session.backend.GetUsersByIDs([]string{userID}) // wgamble@fields.com
			if err != nil {
				fmt.Println(err)
				break
			}
			if len(users) == 0 {
				fmt.Printf("No user found by ID %s\n", userID)
				break
			}
			session.loggedUser = users[0]
			session.selectedList = model.List{}
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

			var waitGroup sync.WaitGroup

			guestsChan := make(chan []model.Guest, len(lists))
			errorsChan := make(chan error, len(lists))

			for _, list := range lists {
				waitGroup.Add(1)
				go func(waitGroup *sync.WaitGroup, list model.List) {
					guests, err := session.backend.GetGuestsByListID(list.ID)
					if err != nil {
						errorsChan <- err
					} else {
						errorsChan <- nil
						guestsChan <- guests
					}
					waitGroup.Done()
				}(&waitGroup, list)
			}
			waitGroup.Wait()
			var errorStr string
			close(errorsChan)
			close(guestsChan)
			for err := range errorsChan {
				if err != nil {
					errorStr += fmt.Sprintf("%v\n", err)
				}
			}
			if errorStr != "" {
				fmt.Print(errorStr)
				break
			}
			for guests := range guestsChan {
				for i, list := range lists {
					if len(guests) > 0 && guests[0].ListID == list.ID {
						lists[i].AggregateGuestCount = len(guests)
					}
				}
			}

			//lists[i].AggregateGuestCount = len(guests)
			if len(lists) > 0 {
				fmt.Printf("%-4s %-37s  %-4s  %-50s\n", "Seq", "ListID", "Guests", "Title")
				fmt.Printf("%-4s %-37s  %-4s  %-50s\n", "---", "------", "------", "-----")
				for _, list := range lists {
					fmt.Printf("%03d  %-37s  %02d      %-50s\n", session.sequenceCounter, list.ID, list.AggregateGuestCount, list.Title)
					session.sequenceList[session.sequenceCounter] = list.ID
					session.sequenceCounter++
				}
				fmt.Println("---")
				fmt.Printf("Use Seq numbers in lieu of IDs. For example, 'list $%d'\n", session.sequenceCounter-1)
			}

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

		// Guests
		case strings.HasPrefix(text, "guests"):
			if session.loggedUser.ID == "" {
				fmt.Println("This command requires a logged user via the 'login' command")
				break
			}
			if session.selectedList.ID == "" {
				fmt.Println("This command requires a a selected list via the 'list select' command")
				break
			}
			guests, err := session.backend.GetGuestsByListID(session.selectedList.ID)
			if err != nil {
				fmt.Println(err)
				break
			}
			if len(guests) > 0 {
				fmt.Printf("%-4s %-37s  %-50s\n", "$n", "UserID", "Email")
				fmt.Printf("%-4s %-37s  %-50s\n", "--", "------", "-----")
				for i, guest := range guests {
					fmt.Printf("%2d   %-37s  %-50s\n", i, guest.UserID, guest.AggregateEmail)
				}
				fmt.Println("Type $n (for example $1) to select a list")
			}

		// List Select
		case strings.HasPrefix(text, "list"):
			if session.loggedUser.ID == "" {
				fmt.Println("This command requires a logged user via the login command")
				break
			}
			if len(text) < len("list _") {
				fmt.Println("No list specified")
				break
			}
			listID := text[len("list "):]
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

	inputLoop(&UserSession{
		backend: backend,
	})

}
