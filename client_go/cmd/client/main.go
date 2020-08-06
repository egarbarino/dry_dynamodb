package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/buger/goterm"
	"github.com/chzyer/readline"
	"github.com/egarbarino/dry_dynamodb/client_go/internal/database/dynamo"
	"github.com/egarbarino/dry_dynamodb/client_go/internal/database/memory"
	"github.com/egarbarino/dry_dynamodb/client_go/internal/model"
	"syreclabs.com/go/faker"
)

const maxResults = 5

// UserSession maintains contextual settings and data
type UserSession struct {
	backend          model.Interface
	loggedUser       model.User
	selectedList     model.List
	lastEvaluatedKey string
	lastCommand      string
	sequenceList     []string
	sequenceCounter  int
	itemVersions     map[string]int
	createRatio      int
	updateRatio      int
	tickRatio        int
}

func simulateInteraction(session *UserSession, threads int, runs int) error {

	if threads < 1 {
		threads = 1
	}
	if runs < 1 {
		runs = 1
	}

	var mutex sync.Mutex
	var currentItems []model.Item
	var globalCounter int32 = 0
	var itemCounter int32 = 0
	var itemErrorCounter int32 = 0
	var createCounter int32 = 0
	var createErrorCounter int32 = 0
	var deleteCounter int32 = 0
	var deleteErrorCounter int32 = 0
	var tickCounter int32 = 0
	var tickErrorCounter int32 = 0
	var updateCounter int32 = 0
	var updateErrorCounter int32 = 0

	interact := func(session *UserSession) {
		for i := 0; i < runs; i++ {
			items, err := session.backend.GetItemsByListID(session.selectedList.ID)
			if err != nil {
				atomic.AddInt32(&itemErrorCounter, 1)
				atomic.AddInt32(&globalCounter, 1)
				log.Printf("GetItemsByListID Error: %v", err)
			}
			mutex.Lock()
			currentItems = items
			mutex.Unlock()
			atomic.AddInt32(&itemCounter, 1)
			atomic.AddInt32(&globalCounter, 1)

			if len(items) < 2 {
				errorStr := "Aborting: Please set up a list with a least five items!"
				fmt.Println(errorStr)
				panic(errorStr)
			}

			randomItem := items[rand.Intn(len(items))]

			randomNumber := rand.Intn(100)
			switch {
			// Create a new item and delete some other from the list
			case session.createRatio != 0 && randomNumber < session.createRatio:

				if err := session.backend.DeleteItem(session.selectedList.ID, randomItem.Datetime); err != nil {
					atomic.AddInt32(&deleteErrorCounter, 1)
					atomic.AddInt32(&globalCounter, 1)
					log.Printf("DeleteItem Error: %v", err)
					break
				}
				atomic.AddInt32(&deleteCounter, 1)
				atomic.AddInt32(&globalCounter, 1)

				for j := 0; j < 5; j++ {
					description := faker.Hacker().Verb() + " " + faker.Hacker().Noun()
					if err := session.backend.CreateItem(session.selectedList.ID, description); err != nil {
						atomic.AddInt32(&createErrorCounter, 1)
						atomic.AddInt32(&globalCounter, 1)
						log.Printf("CreateItem Error Attempt #%d: %v", j, err)
					} else {
						break
					}
				}
				atomic.AddInt32(&createCounter, 1)
				atomic.AddInt32(&globalCounter, 1)

			// Update Description
			case session.updateRatio != 0 && randomNumber >= session.createRatio && randomNumber < session.createRatio+session.updateRatio:
				description := faker.Hacker().Verb() + " " + faker.Hacker().Noun()
				_, err := session.backend.UpdateItem(session.selectedList.ID, randomItem.Datetime, randomItem.Version, aws.String(description), nil)
				if err != nil {
					atomic.AddInt32(&updateErrorCounter, 1)
					atomic.AddInt32(&globalCounter, 1)
					log.Printf("UpdateItem Error: %v", err)
				}
				atomic.AddInt32(&updateCounter, 1)
				atomic.AddInt32(&globalCounter, 1)

			// Tick/Untick an item
			case session.tickRatio != 0 && randomNumber >= (session.createRatio+session.updateRatio) && randomNumber < session.createRatio+session.updateRatio+session.tickRatio:
				_, err := session.backend.UpdateItem(session.selectedList.ID, randomItem.Datetime, randomItem.Version, nil, aws.Bool(!randomItem.Done))
				if err != nil {
					atomic.AddInt32(&tickErrorCounter, 1)
					atomic.AddInt32(&globalCounter, 1)
					log.Printf("UpdateItem Error: %v", err)
				}
				atomic.AddInt32(&tickCounter, 1)
				atomic.AddInt32(&globalCounter, 1)

			}
		}
	}
	start := time.Now()
	for i := 0; i < threads; i++ {
		go interact(session)
	}
	goterm.Clear()
	goterm.MoveCursor(1, 1)
	goterm.Printf("%s (%s)\n",
		session.loggedUser.Email,
		session.loggedUser.ID)
	goterm.Printf("Threads: %d - Runs per Thread: %d (Total: %d)\n", threads, runs, threads*runs)

	for {
		itemCounter := atomic.LoadInt32(&itemCounter)
		goterm.MoveCursor(1, 3)
		counter := atomic.LoadInt32(&globalCounter)
		elapsed := int32(time.Now().Sub(start).Seconds())
		if elapsed == 0 {
			elapsed = 1
		}
		goterm.Printf("Time: %02d:%02d:%02d | Elpased: %ds | Unique Interactions: %d (%d/sec) \n",
			time.Now().Hour(),
			time.Now().Minute(),
			time.Now().Second(),
			elapsed,
			counter,
			counter/elapsed)

		goterm.Printf("   Runs/Get Items: OK %-4d | ERR: %-4d \n",
			itemCounter,
			atomic.LoadInt32(&itemErrorCounter))
		goterm.Printf("     Create Items: OK %-4d | ERR: %-4d (Ratio: %d%%) [Includes Delete]  \n",
			atomic.LoadInt32(&createCounter),
			atomic.LoadInt32(&createErrorCounter),
			session.createRatio)
		goterm.Printf("     Delete Items: OK %-4d | ERR: %-4d \n",
			atomic.LoadInt32(&deleteCounter),
			atomic.LoadInt32(&deleteErrorCounter))
		goterm.Printf("     Rename Items: OK %-4d | ERR: %-4d (Ratio: %d%%) \n",
			atomic.LoadInt32(&updateCounter),
			atomic.LoadInt32(&updateErrorCounter),
			session.updateRatio)
		goterm.Printf("Tick/Untick Items: OK %-4d | ERR: %-4d (Ratio: %d%%) \n",
			atomic.LoadInt32(&tickCounter),
			atomic.LoadInt32(&tickErrorCounter),
			session.tickRatio)
		goterm.Printf("---------------------------------------------------------------\n")
		goterm.Printf("%-27s %-7s %-7s %s\n", "Datetime", "Version", "Done", "Description")
		goterm.Printf("%-27s %-7s %-7s %s\n", "--------", "-------", "----", "-----------")
		var done string
		for _, item := range currentItems {
			if item.Done {
				done = "Done"
			} else {
				done = "Pending"
			}
			goterm.Printf("%-27s %7d %-7s %-40s\n", item.Datetime, item.Version, done, item.Description)
		}
		for j := 1; j < 5; j++ {
			goterm.Printf("%-65s\n", "")
		}
		goterm.Flush()
		time.Sleep(100 * time.Millisecond)

		if int(itemCounter) >= threads*runs {
			fmt.Println("Ending (5 second cool down)")
			time.Sleep(5 * time.Second)
			break
		}

	}
	return nil
}

func help() {
	fmt.Print("" +
		"General Options:\n" +
		"   help                                 Show options\n" +
		"   users                                List users\n" +
		"   user UserID                          Select UserID\n" +
		"   email user@domain.com                Select User by Email\n" +
		"   seq                                  Reset sequence counter\n" +
		"   slow SECONDS                         Delay DB operations\n" +
		"   exit                                 Exit application\n" +
		"Once a user is selected\n" +
		"   lists                                Show User's To Do lists\n" +
		"   list ListID                          Select a List\n" +
		"   list create NAME                     Create a new list\n" +
		"   list delete ListID                   Delete existing list\n" +
		"Once a list is selected\n" +
		"   guests                               List guests invited to the list\n" +
		"   guest add UserID                     Add a guest to the list\n" +
		"   guest remove UserID                  Remove guest from the list\n" +
		"   items                                Show items in the list\n" +
		"   item create DESCRIPTION              Create a new item\n" +
		"   item delete DATETIME                 Delete item by datetimem\n" +
		"   item tick DATETIME                   Set item as done\n" +
		"   item untick DATETIME                 Set item as pending\n" +
		"   item rename DATETIME DESCRIPTION     Change item's description\n" +
		"   interact THREADS RUNS_PER_THREAD     Interact with list automatically\n" +
		"   ratio CREATE UPDATE TICK_UNTICK      Ratio (integer) for interact actions\n")
}

func inputLoop(session *UserSession) {

	session.sequenceCounter = 0
	session.sequenceList = make([]string, 1000)
	session.itemVersions = make(map[string]int)
	var promptStr string
	rl, err := readline.New("> ")
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	for {
		promptStr = ""
		if session.loggedUser.Email != "" {
			promptStr += session.loggedUser.Email
		}
		if session.selectedList.Title != "" {
			promptStr += fmt.Sprintf("|%s", session.selectedList.Title)
		}
		promptStr += "> "
		rl.SetPrompt(promptStr)

		text, err := rl.Readline()
		if err != nil { // io.EOF
			break
		}

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

		case strings.HasPrefix(text, "slow"):
			if len(text) < len("slow _") {
				fmt.Println("No arguments provided")
				break
			}
			argumentStr := text[len("slow "):]
			if n, err := strconv.Atoi(argumentStr); err == nil {
				session.backend.Slowdown(n)
			} else {
				fmt.Printf("%s is not a number\n", argumentStr)
			}
			break

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
					fmt.Printf("%3d  %-37s  %-50s\n", session.sequenceCounter, user.ID, user.Email)
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
			user, err := session.backend.GetUserByEmail(email)
			if err != nil {
				fmt.Println(err)
				break
			}
			session.loggedUser = user
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
				fmt.Println("This command requires a current user via the 'user UserID' command")
				break
			}
			lists, err := session.backend.GetAggregateListsByUserID(session.loggedUser.ID)
			if err != nil {
				fmt.Println(err)
				break
			}
			if len(lists) > 0 {

				var listType string

				fmt.Printf("%-3s %-36s %-5s %-6s %-5s %-50s\n", "Seq", "ListID", "Type", "Guests", "Items", "Title")
				fmt.Printf("%-3s %-36s %-5s %-6s %-5s %-50s\n", "---", "------", "----", "------", "-----", "-----")

				for _, list := range lists {
					if list.AsGuest {
						listType = "Guest"
					} else {
						listType = "Owner"
					}
					fmt.Printf("%3d %-36s %-5s %6d %5d %-50s\n", session.sequenceCounter, list.ID, listType, list.GuestCount, list.ItemCount, list.Title)
					session.sequenceList[session.sequenceCounter] = list.ID
					session.sequenceCounter++
				}
				fmt.Println("---")
				fmt.Printf("Use Seq numbers in lieu of IDs. For example, 'list $%d'\n", session.sequenceCounter-1)
			}

		// List Create
		case strings.HasPrefix(text, "list create"):
			if session.loggedUser.ID == "" {
				fmt.Println("This command requires a current user via the 'user UserID' command")
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
				fmt.Println("This command requires a current user via the 'user UserID' command")
				break
			}
			if len(text) < len("list delete _") {
				fmt.Println("No list Id specified")
				break
			}
			listID := text[len("list delete "):]

			list, err := session.backend.GetListByListID(listID)
			if err != nil {
				fmt.Println(err)
			}
			if list.UserID != session.loggedUser.ID {
				fmt.Println("This list doesn't belong to you!")
				break
			}
			if err := session.backend.DeleteList(listID, session.loggedUser.ID); err != nil {
				fmt.Println(err)
				break
			}
			fmt.Printf("List %s deleted\n", listID)
			if listID == session.selectedList.ID {
				session.selectedList = model.List{}
			}

		// Guests
		case strings.HasPrefix(text, "guests"):
			if session.loggedUser.ID == "" {
				fmt.Println("This command requires a current user via the 'user UserID' command")
				break
			}
			if session.selectedList.ID == "" {
				fmt.Println("This command requires a selected list via the 'list ListID' command")
				break
			}
			guests, err := session.backend.GetAggregateGuestsByListID(session.selectedList.ID)
			if err != nil {
				fmt.Println(err)
				break
			}
			if len(guests) > 0 {
				fmt.Printf("%-4s %-37s  %-50s\n", "Seq", "UserID", "Email")
				fmt.Printf("%-4s %-37s  %-50s\n", "---", "------", "-----")
				for _, guest := range guests {
					fmt.Printf("%3d  %-37s  %-50s\n", session.sequenceCounter, guest.UserID, guest.Email)
					session.sequenceList[session.sequenceCounter] = guest.UserID
					session.sequenceCounter++

				}
				fmt.Println("---")
				fmt.Printf("Use Seq numbers in lieu of IDs. For example, 'guest remove $%d'\n", session.sequenceCounter-1)

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
				isPresent, err := session.backend.IsPresentGuest(listID, session.loggedUser.ID)
				if err != nil {
					fmt.Println(err)
				}
				if !isPresent {
					fmt.Println("This list does not belong to you and you are not a guest in it either!")
					break
				}
			}
			session.selectedList = list

		// Add Guest
		case strings.HasPrefix(text, "guest add"):
			if session.loggedUser.ID == "" {
				fmt.Println("This command requires a current user via the 'user UserID' command")
				break
			}
			if session.selectedList.ID == "" {
				fmt.Println("This command requires a selected list via the 'list ListID' command")
				break
			}
			if len(text) < len("guest add _") {
				fmt.Println("No guest specified")
				break
			}
			userID := text[len("guest add "):]
			if userID == session.loggedUser.ID {
				fmt.Println("You can't add yourself to the guest list")
				break
			}
			err := session.backend.CreateGuest(session.selectedList.ID, userID)
			if err != nil {
				fmt.Println(err)
				break
			}

		case strings.HasPrefix(text, "guest remove"):
			if session.loggedUser.ID == "" {
				fmt.Println("This command requires a current user via the 'user UserID' command")
				break
			}
			if session.selectedList.ID == "" {
				fmt.Println("This command requires a selected list via the 'list ListID' command")
				break
			}
			if len(text) < len("guest remove _") {
				fmt.Println("No guest specified")
				break
			}
			userID := text[len("guest remove "):]
			err := session.backend.DeleteGuest(session.selectedList.ID, userID) // CHECK IF USER EXISTS!!!!
			if err != nil {
				fmt.Println(err)
			}

		case strings.HasPrefix(text, "items"):
			if session.loggedUser.ID == "" {
				fmt.Println("This command requires a current user via the 'user UserID' command")
				break
			}
			if session.selectedList.ID == "" {
				fmt.Println("This command requires a selected list via the 'list ListID' command")
				break
			}
			items, err := session.backend.GetItemsByListID(session.selectedList.ID)
			if err != nil {
				fmt.Println(err)
				break
			}
			if len(items) > 0 {

				var done string

				fmt.Printf("%-3s %-27s %-7s %-7s %s\n", "Seq", "Datetime", "Version", "Done", "Description")
				fmt.Printf("%-3s %-27s %-7s %-7s %s\n", "---", "--------", "-------", "----", "-----------")

				for _, item := range items {
					if item.Done {
						done = "Done"
					} else {
						done = "Pending"
					}

					fmt.Printf("%3d %-27s %7d %-7s %s\n", session.sequenceCounter, item.Datetime, item.Version, done, item.Description)
					session.itemVersions[item.Datetime] = item.Version
					session.sequenceList[session.sequenceCounter] = item.Datetime
					session.sequenceCounter++
				}
				fmt.Println("---")
				fmt.Printf("Use Seq numbers in lieu of IDs. For example, 'item delete $%d'\n", session.sequenceCounter-1)
			}

		case strings.HasPrefix(text, "item create"):
			if session.loggedUser.ID == "" {
				fmt.Println("This command requires a current user via the 'user UserID' command")
				break
			}
			if session.selectedList.ID == "" {
				fmt.Println("This command requires a selected list via the 'list ListID' command")
				break
			}
			if len(text) < len("item create _") {
				fmt.Println("No description specified")
				break
			}
			description := text[len("item create "):]
			err := session.backend.CreateItem(session.selectedList.ID, description)
			if err != nil {
				fmt.Println(err)
				break
			}

		case strings.HasPrefix(text, "item delete"):
			if session.loggedUser.ID == "" {
				fmt.Println("This command requires a current user via the 'user UserID' command")
				break
			}
			if session.selectedList.ID == "" {
				fmt.Println("This command requires a selected list via the 'list ListID' command")
				break
			}
			if len(text) < len("item delete _") {
				fmt.Println("No datetime specified")
				break
			}
			datetime := text[len("item delete "):]
			err := session.backend.DeleteItem(session.selectedList.ID, datetime)
			if err != nil {
				fmt.Println(err)
				break
			}

		case strings.HasPrefix(text, "item tick"):
			if session.loggedUser.ID == "" {
				fmt.Println("This command requires a current user via the 'user UserID' command")
				break
			}
			if session.selectedList.ID == "" {
				fmt.Println("This command requires a selected list via the 'list ListID' command")
				break
			}
			if len(text) < len("item tick _") {
				fmt.Println("No datetime specified")
				break
			}
			datetime := text[len("item tick "):]
			newVersion, err := session.backend.UpdateItem(session.selectedList.ID, datetime, session.itemVersions[datetime], nil, aws.Bool(true))
			if err != nil {
				fmt.Println(err)
				break
			}
			session.itemVersions[datetime] = newVersion

		case strings.HasPrefix(text, "item untick"):
			if session.loggedUser.ID == "" {
				fmt.Println("This command requires a current user via the 'user UserID' command")
				break
			}
			if session.selectedList.ID == "" {
				fmt.Println("This command requires a selected list via the 'list ListID' command")
				break
			}
			if len(text) < len("item untick _") {
				fmt.Println("No datetime specified")
				break
			}
			datetime := text[len("item untick "):]
			newVersion, err := session.backend.UpdateItem(session.selectedList.ID, datetime, session.itemVersions[datetime], nil, aws.Bool(false))
			if err != nil {
				fmt.Println(err)
				break
			}
			session.itemVersions[datetime] = newVersion

		case strings.HasPrefix(text, "item rename"):
			if session.loggedUser.ID == "" {
				fmt.Println("This command requires a current user via the 'user UserID' command")
				break
			}
			if session.selectedList.ID == "" {
				fmt.Println("This command requires a selected list via the 'list ListID' command")
				break
			}
			if len(text) < len("item rename _") {
				fmt.Println("No arguments provided")
				break
			}
			argumentStr := text[len("item rename "):]
			arguments := strings.Split(argumentStr, " ")
			if len(arguments) < 2 {
				fmt.Println("Invalid number of arguments")
				break
			}
			datetime := arguments[0]
			description := strings.Join(arguments[1:], " ")
			newVersion, err := session.backend.UpdateItem(session.selectedList.ID, datetime, session.itemVersions[datetime], &description, nil)
			if err != nil {
				fmt.Println(err)
				break
			}
			session.itemVersions[datetime] = newVersion

		case strings.HasPrefix(text, "interact"):
			if session.loggedUser.ID == "" {
				fmt.Println("This command requires a current user via the 'user UserID' command")
				break
			}
			if session.selectedList.ID == "" {
				fmt.Println("This command requires a selected list via the 'list ListID' command")
				break
			}
			if len(text) < len("interact _") {
				fmt.Println("No arguments provided")
				break
			}
			argumentStr := text[len("interact "):]
			arguments := strings.Split(argumentStr, " ")
			if len(arguments) < 2 {
				fmt.Println("Invalid number of arguments")
				break
			}
			var threads int
			var runs int
			if n, err := strconv.Atoi(arguments[0]); err == nil {
				threads = n
			} else {
				fmt.Printf("%s is not a number\n", arguments[0])
			}
			if n, err := strconv.Atoi(arguments[1]); err == nil {
				runs = n
			} else {
				fmt.Printf("%s is not a number\n", arguments[1])
			}

			if err := simulateInteraction(session, threads, runs); err != nil {
				fmt.Println(err)
				break
			}
		case strings.HasPrefix(text, "ratio"):
			if len(text) < len("ratio _") {
				fmt.Println("No arguments provided")
				break
			}
			argumentStr := text[len("ratio "):]
			arguments := strings.Split(argumentStr, " ")
			if len(arguments) < 3 {
				fmt.Println("Invalid number of arguments")
				break
			}
			if n, err := strconv.Atoi(arguments[0]); err == nil {
				session.createRatio = n
			} else {
				fmt.Printf("%s is not a number\n", arguments[0])
			}

			if n, err := strconv.Atoi(arguments[1]); err == nil {
				session.updateRatio = n
			} else {
				fmt.Printf("%s is not a number\n", arguments[1])
			}

			if n, err := strconv.Atoi(arguments[2]); err == nil {
				session.tickRatio = n
			} else {
				fmt.Printf("%s is not a number\n", arguments[2])
			}

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

	// Abstract interface
	var backend model.Interface

	fmt.Print("*** Todo List Application ***\n\n")
	fmt.Print("Usage: ./client memory | ./client (default using DynamoDB)\n")
	if len(os.Args) > 1 && os.Args[1] == "memory" {
		fmt.Print("\nMemory backend selected\n\n")

		// Use Memory Implementation
		backend = memory.New()

	} else {
		fmt.Print("\nDynamoDB backend selected\n\n")

		var session *session.Session = session.Must(
			session.NewSessionWithOptions(
				session.Options{
					Profile: "dynamodb_profile",
					Config: aws.Config{
						Region: aws.String("eu-west-2"),
					},
				}))

		// Use DynamoDB Implementation
		backend = &dynamo.DBSession{DynamoDBresource: dynamodb.New(session)}

	}

	help()
	inputLoop(&UserSession{
		backend: backend,
	})

}
