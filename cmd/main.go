package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type SecretResponse struct {
	SecretString string `json:"SecretString"`
}

type DozorCode struct {
	Code     string
	Room     string
	Username string
	Note     string
}

func getBotToken() (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://localhost:2773/secretsmanager/get?secretId=BotToken", nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("X-Aws-Parameters-Secrets-Token", os.Getenv("AWS_SESSION_TOKEN"))

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	var secret SecretResponse
	if err := json.NewDecoder(resp.Body).Decode(&secret); err != nil {
		return "", err
	}

	return secret.SecretString, nil
}

func setCommandsMenu(bot *tgbotapi.BotAPI) error {
	commands := []tgbotapi.BotCommand{
		{
			Command:     "register",
			Description: "set your username",
		},
		{
			Command:     "team",
			Description: "set your team",
		},
		{
			Command:     "code",
			Description: "send the code",
		},
		{
			Command:     "codes",
			Description: "get the codes",
		},
		{
			Command:     "top",
			Description: "get the top",
		},
		{
			Command:     "a3",
			Description: "send the answer for a3",
		},
		{
			Command:     "b1",
			Description: "send the answer for b1",
		},
		{
			Command:     "what",
			Description: "get the list of commands",
		},
		{
			Command:     "whoami",
			Description: "get your username and team",
		},
	}
	if _, err := bot.Request(tgbotapi.NewSetMyCommands(commands...)); err != nil {
		log.Printf("failed to set commands: %v\n", err)
		return err
	}
	return nil
}

func registerUsername(bot *tgbotapi.BotAPI, svc *dynamodb.DynamoDB, fromID int64, chatID int64, username string) error {
	if username == "" {
		msg := tgbotapi.NewMessage(chatID, "Please provide a username")
		bot.Send(msg)
		return nil
	}

	// check if the username with this chatID already exists
	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String("UserProfile"),
		Key: map[string]*dynamodb.AttributeValue{
			"from_id": {
				N: aws.String(fmt.Sprint(fromID)),
			},
		},
	})
	if err != nil {
		log.Printf("failed to get item: %v\n", err)
		return err
	}

	if result.Item != nil {
		// update the username
		_, err := svc.UpdateItem(&dynamodb.UpdateItemInput{
			TableName: aws.String("UserProfile"),
			Key: map[string]*dynamodb.AttributeValue{
				"from_id": {
					N: aws.String(fmt.Sprint(fromID)),
				},
			},
			UpdateExpression: aws.String("set username = :u"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":u": {
					S: aws.String(username),
				},
			},
		})
		if err != nil {
			log.Printf("failed to update item: %v\n", err)
			return err
		}
	} else {
		// create a new item
		_, err := svc.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String("UserProfile"),
			Item: map[string]*dynamodb.AttributeValue{
				"from_id": {
					N: aws.String(fmt.Sprint(fromID)),
				},
				"username": {
					S: aws.String(username),
				},
			},
		})
		if err != nil {
			log.Printf("failed to put item: %v\n", err)
			return err
		}
	}

	msg := tgbotapi.NewMessage(chatID, "Nice to meet you, "+username+"!")
	bot.Send(msg)
	return nil
}

// valid teams are 'A', 'B, 'C', 'D'
var validStrings = []string{"A", "B", "C", "D"}

func isValidTeam(team string) bool {
	for _, valid := range validStrings {
		if valid == team {
			return true
		}
	}
	return false
}

func registerTeam(bot *tgbotapi.BotAPI, svc *dynamodb.DynamoDB, fromID int64, chatID int64, team string) error {
	if !isValidTeam(team) {
		validTeams := ""
		for _, valid := range validStrings {
			validTeams += "'" + valid + "' "
		}
		msg := tgbotapi.NewMessage(chatID, "Please provide a valid team. Valid teams are "+validTeams)
		bot.Send(msg)
		return nil
	}

	if ok, err := isRegistered(svc, fromID); !ok || err != nil {
		msg := tgbotapi.NewMessage(chatID, "Please register first")
		bot.Send(msg)
		return nil
	}

	// update the user item with the team
	_, err := svc.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String("UserProfile"),
		Key: map[string]*dynamodb.AttributeValue{
			"from_id": {
				N: aws.String(fmt.Sprint(fromID)),
			},
		},
		UpdateExpression: aws.String("set team = :t"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":t": {
				S: aws.String(team),
			},
		},
	})
	if err != nil {
		log.Printf("failed to update item: %v\n", err)
		return err
	}

	msg := tgbotapi.NewMessage(chatID, "Welcome to team "+team+"!")
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	bot.Send(msg)
	return nil
}

func isRegistered(svc *dynamodb.DynamoDB, fromID int64) (bool, error) {
	// check if the username with this fromID already exists
	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String("UserProfile"),
		Key: map[string]*dynamodb.AttributeValue{
			"from_id": {
				N: aws.String(fmt.Sprint(fromID)),
			},
		},
	})
	if err != nil {
		log.Printf("failed to get item: %v\n", err)
		return false, err
	}

	if result.Item != nil {
		return true, nil
	}

	return false, nil
}

func getUsername(svc *dynamodb.DynamoDB, fromID int64) (string, error) {
	// check if the username with this fromID already exists
	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String("UserProfile"),
		Key: map[string]*dynamodb.AttributeValue{
			"from_id": {
				N: aws.String(fmt.Sprint(fromID)),
			},
		},
	})
	if err != nil {
		log.Printf("failed to get item: %v\n", err)
		return "", err
	}

	if result.Item != nil {
		if result.Item["username"] != nil {
			return *result.Item["username"].S, nil
		}
	}

	return "", nil
}

func getTeam(svc *dynamodb.DynamoDB, fromID int64) (string, error) {
	// check if the username with this fromID already exists
	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String("UserProfile"),
		Key: map[string]*dynamodb.AttributeValue{
			"from_id": {
				N: aws.String(fmt.Sprint(fromID)),
			},
		},
	})
	if err != nil {
		log.Printf("failed to get item: %v\n", err)
		return "", err
	}

	if result.Item != nil {
		if result.Item["team"] != nil {
			return *result.Item["team"].S, nil
		}
	}

	return "", nil
}

var adminSecret = "9cfc73c0ff8498aa083c2be9c7449f7894e9c0a9621422fec74c3361ab8633dc"

func calculateHash(input string) string {
	// Create a new hash.Hash object using SHA-256
	hasher := sha256.New()

	// Write the input string to the hash
	hasher.Write([]byte(input))

	// Get the final hash as a byte slice
	hashBytes := hasher.Sum(nil)

	// Convert the byte slice to a hexadecimal string
	hashString := hex.EncodeToString(hashBytes)

	return hashString
}

func updateAdmin(bot *tgbotapi.BotAPI, svc *dynamodb.DynamoDB, fromID int64, chatID int64, secret string, enabled bool) error {
	if ok, err := isRegistered(svc, fromID); !ok || err != nil {
		msg := tgbotapi.NewMessage(chatID, "Please register first")
		bot.Send(msg)
		return nil
	}

	if calculateHash(secret) != adminSecret {
		msg := tgbotapi.NewMessage(chatID, "You are not an admin")
		bot.Send(msg)
		return nil
	}

	// update the user with a new attribute 'admin'
	_, err := svc.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String("UserProfile"),
		Key: map[string]*dynamodb.AttributeValue{
			"from_id": {
				N: aws.String(fmt.Sprint(fromID)),
			},
		},
		UpdateExpression: aws.String("set admin = :a"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":a": {
				BOOL: aws.Bool(enabled),
			},
		},
	})
	if err != nil {
		log.Printf("failed to update item: %v\n", err)
		return err
	}

	messageString := ""
	if enabled {
		messageString = "You are now an admin"
	} else {
		messageString = "You are not an admin anymore"
	}
	msg := tgbotapi.NewMessage(chatID, messageString)
	bot.Send(msg)
	return nil
}

func isAdmin(svc *dynamodb.DynamoDB, fromID int64) (bool, error) {
	// check if the username with this fromID already exists
	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String("UserProfile"),
		Key: map[string]*dynamodb.AttributeValue{
			"from_id": {
				N: aws.String(fmt.Sprint(fromID)),
			},
		},
	})
	if err != nil {
		log.Printf("failed to get item: %v\n", err)
		return false, err
	}

	if result.Item != nil {
		if result.Item["admin"] != nil {
			return *result.Item["admin"].BOOL, nil
		}
	}

	return false, nil
}

func addCode(bot *tgbotapi.BotAPI, svc *dynamodb.DynamoDB, fromID int64, chatID int64, commandArgument string) error {
	if ok, err := isAdmin(svc, fromID); !ok || err != nil {
		msg := tgbotapi.NewMessage(chatID, "You are not an admin")
		bot.Send(msg)
		return nil
	}

	codeString, roomString, noteString := "", "", ""

	// parse with delimeter '-'
	if len(commandArgument) > 0 {
		arguments := strings.Split(commandArgument, "-")
		if len(arguments) > 0 {
			codeString = strings.TrimSpace(arguments[0])
		}
		if len(arguments) > 1 {
			roomString = strings.TrimSpace(arguments[1])
		}
		if len(arguments) > 2 {
			noteString = strings.TrimSpace(arguments[2])
		}
	}

	if codeString == "" {
		msg := tgbotapi.NewMessage(chatID, "Please provide a code")
		bot.Send(msg)
		return nil
	}

	if roomString == "" {
		msg := tgbotapi.NewMessage(chatID, "Please provide a room")
		bot.Send(msg)
		return nil
	}

	dozorCode, err := getCode(svc, codeString)
	if err != nil {
		log.Printf("failed to get code: %v\n", err)
		return err
	}
	if dozorCode != nil {
		// update the code item with the room and note
		_, err := svc.UpdateItem(&dynamodb.UpdateItemInput{
			TableName: aws.String("DozorCode"),
			Key: map[string]*dynamodb.AttributeValue{
				"code": {
					S: aws.String(codeString),
				},
			},
			UpdateExpression: aws.String("set room = :r, note = :n"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":r": {
					S: aws.String(roomString),
				},
				":n": {
					S: aws.String(noteString),
				},
			},
		})
		if err != nil {
			log.Printf("failed to update item: %v\n", err)
			return err
		}

		codeMessage := "Code " + codeString + " was updated with room " + roomString
		if noteString != "" {
			codeMessage += " with note " + noteString
		}
		msg := tgbotapi.NewMessage(chatID, codeMessage)
		bot.Send(msg)
	} else {
		// create a new code item
		_, err = svc.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String("DozorCode"),
			Item: map[string]*dynamodb.AttributeValue{
				"code": {
					S: aws.String(codeString),
				},
				"room": {
					S: aws.String(roomString),
				},
				"note": {
					S: aws.String(noteString),
				},
			},
		})
		if err != nil {
			log.Printf("failed to put item: %v\n", err)
			return err
		}

		codeMessage := "Code " + codeString + " was added to room " + roomString
		if noteString != "" {
			codeMessage += " with note " + noteString
		}
		msg := tgbotapi.NewMessage(chatID, codeMessage)
		bot.Send(msg)
	}

	return nil
}

func sendCode(bot *tgbotapi.BotAPI, svc *dynamodb.DynamoDB, fromID int64, chatID int64, codeString string) error {
	if ok, err := isRegistered(svc, fromID); !ok || err != nil {
		msg := tgbotapi.NewMessage(chatID, "Please register first")
		bot.Send(msg)
		return nil
	}

	if codeString == "" {
		msg := tgbotapi.NewMessage(chatID, "Please provide a code")
		bot.Send(msg)
		return nil
	}

	dozorCode, err := getCode(svc, codeString)
	if err != nil {
		log.Printf("failed to get code: %v\n", err)
		return err
	}
	if dozorCode == nil {
		msg := tgbotapi.NewMessage(chatID, "Code "+codeString+" does not exist")
		bot.Send(msg)
		return nil
	}
	if dozorCode.Username != "" {
		// already found by someone
		msg := tgbotapi.NewMessage(chatID, "Code "+codeString+" was already found by "+dozorCode.Username)
		bot.Send(msg)
	}

	// update the code with the fromID
	_, err = svc.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String("DozorCode"),
		Key: map[string]*dynamodb.AttributeValue{
			"code": {
				S: aws.String(codeString),
			},
		},
		UpdateExpression: aws.String("set from_id = :f"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":f": {
				N: aws.String(fmt.Sprint(fromID)),
			},
		},
	})
	if err != nil {
		log.Printf("failed to update item: %v\n", err)
		return err
	}

	username, err := getUsername(svc, fromID)
	if err != nil {
		log.Printf("failed to get username: %v\n", err)
		return err
	}

	messageString := "Congratulations, " + username + "! You found the code " + codeString
	msg := tgbotapi.NewMessage(chatID, messageString)
	bot.Send(msg)
	return nil
}

func listCodes(bot *tgbotapi.BotAPI, svc *dynamodb.DynamoDB, fromID int64, chatID int64) error {
	if ok, err := isRegistered(svc, fromID); !ok || err != nil {
		msg := tgbotapi.NewMessage(chatID, "Please register first")
		bot.Send(msg)
		return nil
	}

	isUserAdmin, err := isAdmin(svc, fromID)
	if err != nil {
		log.Printf("failed to check if user is admin: %v\n", err)
		isUserAdmin = false
	}

	// get all codes
	result, err := svc.Scan(&dynamodb.ScanInput{
		TableName: aws.String("DozorCode"),
	})
	if err != nil {
		log.Printf("failed to scan table: %v\n", err)
		return err
	}

	dozorCodesByRoom := make(map[string][]*DozorCode)
	for _, item := range result.Items {
		dozorCode := &DozorCode{
			Code: *item["code"].S,
		}
		if item["room"] != nil {
			dozorCode.Room = *item["room"].S
		}
		if item["from_id"] != nil {
			fromIDStr := *item["from_id"].N
			// convert fromIDStr to int64
			fromID, err := strconv.ParseInt(fromIDStr, 10, 64)
			if err != nil {
				log.Printf("failed to parse from_id: %v\n", err)
			} else {
				dozorCode.Username, err = getUsername(svc, fromID)
				if err != nil {
					log.Printf("failed to get username: %v\n", err)
				}
			}
		}
		if item["note"] != nil {
			dozorCode.Note = *item["note"].S
		}
		if dozorCode.Username != "" {
			// if the code was found by the user, put it first
			dozorCodesByRoom[dozorCode.Room] = append([]*DozorCode{dozorCode}, dozorCodesByRoom[dozorCode.Room]...)
		} else {
			// if the code was not found by the user, put it last
			dozorCodesByRoom[dozorCode.Room] = append(dozorCodesByRoom[dozorCode.Room], dozorCode)
		}
	}

	codes := ""
	foundCount := 0
	totalCount := 0
	for room, dozorCodes := range dozorCodesByRoom {
		if dozorCodes == nil || len(dozorCodes) == 0 {
			continue
		}
		codes += room + ":\n"
		notFoundCount := 0
		for _, dozorCode := range dozorCodes {
			if !isUserAdmin && dozorCode.Username == "" {
				notFoundCount++
				continue
			}

			codes += dozorCode.Code + " "
			if dozorCode.Username != "" {
				codes += "found by " + dozorCode.Username + " "
			}
			if isUserAdmin && dozorCode.Note != "" {
				codes += "note: " + dozorCode.Note + " "
			}
			codes += "\n"

			if dozorCode.Username != "" {
				foundCount++
			}
			totalCount++
		}
		if notFoundCount > 0 {
			codes += "Not found: " + strconv.Itoa(notFoundCount) + " codes\n"
		}
		codes += "\n"
	}

	codes = "Found: " + strconv.Itoa(foundCount) + " codes\n" +
		"Left: " + strconv.Itoa(totalCount-foundCount) + " codes\n" +
		"Total: " + strconv.Itoa(totalCount) + " codes\n\n" +
		codes

	msg := tgbotapi.NewMessage(chatID, codes)
	bot.Send(msg)
	return nil
}

type TopEntry struct {
	Username string
	Teamname string
	Count    int
}

func listTop(bot *tgbotapi.BotAPI, svc *dynamodb.DynamoDB, fromID int64, chatID int64) error {
	if ok, err := isRegistered(svc, fromID); !ok || err != nil {
		msg := tgbotapi.NewMessage(chatID, "Please register first")
		bot.Send(msg)
		return nil
	}

	// get all codes
	result, err := svc.Scan(&dynamodb.ScanInput{
		TableName: aws.String("DozorCode"),
	})
	if err != nil {
		log.Printf("failed to scan table: %v\n", err)
		return err
	}

	dozorCodesByUser := make(map[string][]*DozorCode)
	for _, item := range result.Items {
		dozorCode := &DozorCode{
			Code: *item["code"].S,
		}
		if item["room"] != nil {
			dozorCode.Room = *item["room"].S
		}
		if item["from_id"] != nil {
			fromIDStr := *item["from_id"].N
			// convert fromIDStr to int64
			fromID, err := strconv.ParseInt(fromIDStr, 10, 64)
			if err != nil {
				log.Printf("failed to parse from_id: %v\n", err)
			} else {
				dozorCode.Username, err = getUsername(svc, fromID)
				if err != nil {
					log.Printf("failed to get username: %v\n", err)
				}
			}
		}
		if item["note"] != nil {
			dozorCode.Note = *item["note"].S
		}
		if dozorCode.Username != "" {
			dozorCodesByUser[dozorCode.Username] = append(dozorCodesByUser[dozorCode.Username], dozorCode)
		}
	}

	if len(dozorCodesByUser) == 0 {
		msg := tgbotapi.NewMessage(chatID, "No codes were found yet")
		bot.Send(msg)
		return nil
	}

	topEntries := make([]*TopEntry, 0)
	for username, dozorCodes := range dozorCodesByUser {
		topEntries = append(topEntries, &TopEntry{
			Username: username,
			Count:    len(dozorCodes),
		})
	}

	// sort topEntries by count
	for i := 0; i < len(topEntries); i++ {
		for j := i + 1; j < len(topEntries); j++ {
			if topEntries[i].Count < topEntries[j].Count {
				topEntries[i], topEntries[j] = topEntries[j], topEntries[i]
			}
		}
	}

	// find the team for each user
	for _, topEntry := range topEntries {
		result, err := svc.Scan(&dynamodb.ScanInput{
			TableName:        aws.String("UserProfile"),
			FilterExpression: aws.String("username = :u"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":u": {
					S: aws.String(topEntry.Username),
				},
			},
		})
		if err != nil {
			log.Printf("failed to scan table: %v\n", err)
			continue
		}
		if len(result.Items) > 0 {
			if result.Items[0]["team"] != nil {
				topEntry.Teamname = *result.Items[0]["team"].S
			}
		}
	}

	top := ""
	for i, topEntry := range topEntries {
		top += strconv.Itoa(i+1) + ". " + topEntry.Username + " " + strconv.Itoa(topEntry.Count)
		if topEntry.Teamname != "" {
			top += " (team " + topEntry.Teamname + ")"
		}
		top += "\n"
	}

	msg := tgbotapi.NewMessage(chatID, top)
	bot.Send(msg)

	return nil
}

func removeCode(bot *tgbotapi.BotAPI, svc *dynamodb.DynamoDB, fromID int64, chatID int64, codeString string) error {
	if ok, err := isAdmin(svc, fromID); !ok || err != nil {
		msg := tgbotapi.NewMessage(chatID, "You are not an admin")
		bot.Send(msg)
		return nil
	}

	dozorCode, err := getCode(svc, codeString)
	if err != nil {
		log.Printf("failed to get code: %v\n", err)
		return err
	}
	if dozorCode == nil {
		msg := tgbotapi.NewMessage(chatID, "Code "+codeString+" does not exist")
		bot.Send(msg)
		return nil
	}

	// remove the code
	_, err = svc.DeleteItem(&dynamodb.DeleteItemInput{
		TableName: aws.String("DozorCode"),
		Key: map[string]*dynamodb.AttributeValue{
			"code": {
				S: aws.String(codeString),
			},
		},
	})
	if err != nil {
		log.Printf("failed to delete item: %v\n", err)
		return err
	}

	msg := tgbotapi.NewMessage(chatID, "Code "+codeString+" was removed")
	bot.Send(msg)
	return nil
}

func getCode(svc *dynamodb.DynamoDB, code string) (*DozorCode, error) {
	// check if the code exists
	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String("DozorCode"),
		Key: map[string]*dynamodb.AttributeValue{
			"code": {
				S: aws.String(code),
			},
		},
	})
	if err != nil {
		log.Printf("failed to get item: %v\n", err)
		return nil, err
	}

	if result.Item != nil {
		dozorCode := &DozorCode{
			Code: *result.Item["code"].S,
		}
		if result.Item["room"] != nil {
			dozorCode.Room = *result.Item["room"].S
		}
		if result.Item["note"] != nil {
			dozorCode.Note = *result.Item["note"].S
		}
		if result.Item["from_id"] != nil {
			fromIDStr := *result.Item["from_id"].N
			// convert fromIDStr to int64
			fromID, err := strconv.ParseInt(fromIDStr, 10, 64)
			if err != nil {
				log.Printf("failed to parse from_id: %v\n", err)
			} else {
				dozorCode.Username, err = getUsername(svc, fromID)
				if err != nil {
					log.Printf("failed to get username: %v\n", err)
				}
			}
		}
		return dozorCode, nil
	}

	return nil, nil
}

func answerPair(bot *tgbotapi.BotAPI, svc *dynamodb.DynamoDB, fromID int64, chatID int64, commandArgument string, tablename string) error {
	if ok, err := isRegistered(svc, fromID); !ok || err != nil {
		msg := tgbotapi.NewMessage(chatID, "Please register first")
		bot.Send(msg)
		return nil
	}

	if commandArgument == "" {
		msg := tgbotapi.NewMessage(chatID, "Please provide a answer")
		bot.Send(msg)
		return nil
	}

	// check if all the answers found
	result, err := svc.Scan(&dynamodb.ScanInput{
		TableName: aws.String(tablename),
	})
	if err != nil {
		log.Printf("failed to scan table: %v\n", err)
		return err
	}
	allFound := true
	if result.Items != nil && len(result.Items) > 0 {
		for _, item := range result.Items {
			if item["from_id"] == nil {
				allFound = false
				break
			}
		}
	}
	if allFound {
		msg := tgbotapi.NewMessage(chatID, "All answers were found")
		bot.Send(msg)
		return nil
	}

	// to lower
	commandArgument = strings.ToLower(commandArgument)
	// trim spaces
	commandArgument = strings.TrimSpace(commandArgument)

	// check table tablename to find the item with 'answer' equal to commandArgument
	result, err = svc.Scan(&dynamodb.ScanInput{
		TableName:        aws.String(tablename),
		FilterExpression: aws.String("answer = :a"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":a": {
				S: aws.String(commandArgument),
			},
		},
	})
	if err != nil {
		log.Printf("failed to scan table: %v\n", err)
		return err
	}
	if result.Items == nil || len(result.Items) == 0 {
		msg := tgbotapi.NewMessage(chatID, "Wrong answer")
		bot.Send(msg)
		return nil
	}

	// mark the answer as found
	_, err = svc.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(tablename),
		Key: map[string]*dynamodb.AttributeValue{
			"answer": {
				S: aws.String(commandArgument),
			},
		},
		UpdateExpression: aws.String("set from_id = :f"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":f": {
				N: aws.String(fmt.Sprint(fromID)),
			},
		},
	})
	if err != nil {
		log.Printf("failed to update item: %v\n", err)
		return err
	}

	// check if all the answers found
	result, err = svc.Scan(&dynamodb.ScanInput{
		TableName: aws.String(tablename),
	})
	if err != nil {
		log.Printf("failed to scan table: %v\n", err)
		return err
	}
	allFound = true
	if result.Items != nil && len(result.Items) > 0 {
		for _, item := range result.Items {
			if item["from_id"] == nil {
				allFound = false
				break
			}
		}
	}
	if allFound {
		msg := tgbotapi.NewMessage(chatID, "All answers were found")
		bot.Send(msg)
		return nil
	}

	username, err := getUsername(svc, fromID)
	if err != nil {
		log.Printf("failed to get username: %v\n", err)
		return err
	}

	messageString := "Congratulations, " + username + "! You found the answer " + commandArgument
	msg := tgbotapi.NewMessage(chatID, messageString)
	bot.Send(msg)
	return nil
}

func addPair(bot *tgbotapi.BotAPI, svc *dynamodb.DynamoDB, fromID int64, chatID int64, commandArgument string, tablename string) error {
	if ok, err := isAdmin(svc, fromID); !ok || err != nil {
		msg := tgbotapi.NewMessage(chatID, "You are not an admin")
		bot.Send(msg)
		return nil
	}

	if commandArgument == "" {
		msg := tgbotapi.NewMessage(chatID, "Please provide a answer")
		bot.Send(msg)
		return nil
	}

	// to lower
	commandArgument = strings.ToLower(commandArgument)
	// trim spaces
	commandArgument = strings.TrimSpace(commandArgument)

	// check table tablename to find the item with 'answer' equal to commandArgument
	result, err := svc.Scan(&dynamodb.ScanInput{
		TableName:        aws.String(tablename),
		FilterExpression: aws.String("answer = :a"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":a": {
				S: aws.String(commandArgument),
			},
		},
	})
	if err != nil {
		log.Printf("failed to scan table: %v\n", err)
		return err
	}
	if result.Items != nil && len(result.Items) > 0 {
		msg := tgbotapi.NewMessage(chatID, "Answer "+commandArgument+" already exists")
		bot.Send(msg)
		return nil
	}

	// create a new item
	_, err = svc.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(tablename),
		Item: map[string]*dynamodb.AttributeValue{
			"answer": {
				S: aws.String(commandArgument),
			},
		},
	})
	if err != nil {
		log.Printf("failed to put item: %v\n", err)
		return err
	}

	msg := tgbotapi.NewMessage(chatID, "Answer "+commandArgument+" was added")
	bot.Send(msg)
	return nil
}

func listPair(bot *tgbotapi.BotAPI, svc *dynamodb.DynamoDB, fromID int64, chatID int64, tablename string) error {
	if ok, err := isAdmin(svc, fromID); !ok || err != nil {
		msg := tgbotapi.NewMessage(chatID, "You are not an admin")
		bot.Send(msg)
		return nil
	}

	// get all answers
	result, err := svc.Scan(&dynamodb.ScanInput{
		TableName: aws.String(tablename),
	})
	if err != nil {
		log.Printf("failed to scan table: %v\n", err)
		return err
	}

	answers := ""
	foundCount := 0
	for _, item := range result.Items {
		answerString := ""
		if item["answer"] != nil {
			answerString += *item["answer"].S
		}
		if item["from_id"] != nil {
			fromIDStr := *item["from_id"].N
			finderString := fromIDStr
			// convert fromIDStr to int64
			fromID, err := strconv.ParseInt(fromIDStr, 10, 64)
			if err != nil {
				log.Printf("failed to parse from_id: %v\n", err)
			} else {
				username, err := getUsername(svc, fromID)
				if err != nil {
					log.Printf("failed to get username: %v\n", err)
				} else {
					finderString = username
				}
			}
			answerString += ": found by " + finderString

			// add to the back of the answers
			answers += answerString + "\n"

			foundCount++
		} else {
			answerString += ": not found"

			// add to the front of the answers
			answers = answerString + "\n" + answers
		}

	}

	if answers == "" {
		answers = "No answers were added yet"
	}

	// add found and left count to the beginning
	answers = "Found: " + strconv.Itoa(foundCount) + " answers\n" +
		"Left: " + strconv.Itoa(len(result.Items)-foundCount) + " answers\n\n" +
		answers

	msg := tgbotapi.NewMessage(chatID, answers)
	bot.Send(msg)
	return nil
}

func getWaitingCommand(svc *dynamodb.DynamoDB, fromID int64, messageDate int) (string, error) {
	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String("WaitingCommand"),
		Key: map[string]*dynamodb.AttributeValue{
			"from_id": {
				N: aws.String(fmt.Sprint(fromID)),
			},
		},
	})
	if err != nil {
		log.Printf("failed to get item: %v\n", err)
		return "", err
	}
	if result.Item == nil {
		return "", nil
	}

	// get the command
	command := ""
	if result.Item["command"] != nil {
		command = *result.Item["command"].S
	}

	// get the timestamp
	timestamp := int64(0)
	if result.Item["timestamp"] != nil {
		timestampStr := ""
		if result.Item["timestamp"] != nil {
			timestampStr = *result.Item["timestamp"].N
		}
		// convert timestampStr to int64
		t, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			log.Printf("failed to parse timestamp: %v\n", err)
		} else {
			timestamp = t
		}
	}

	// remove the item from DynamoDB
	_, err = svc.DeleteItem(&dynamodb.DeleteItemInput{
		TableName: aws.String("WaitingCommand"),
		Key: map[string]*dynamodb.AttributeValue{
			"from_id": {
				N: aws.String(fmt.Sprint(fromID)),
			},
		},
	})
	if err != nil {
		log.Printf("failed to delete item: %v\n", err)
	}

	// check if the timestamp is less than 5 minutes
	if timestamp+300 < int64(messageDate) {
		return "", nil
	}

	return command, nil
}

func handler(ctx context.Context, kinesisEvent events.KinesisEvent) error {
	token, err := getBotToken()
	if err != nil {
		log.Fatalf("failed to get bot token: %v", err)
		return err
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("failed to create bot: %v", err)
		return err
	}

	setCommandsMenu(bot)

	// create a DynamoDB client
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String("eu-central-1"),
	}))
	svc := dynamodb.New(sess)

	for _, record := range kinesisEvent.Records {
		var update tgbotapi.Update
		if err := json.Unmarshal([]byte(record.Kinesis.Data), &update); err != nil {
			log.Println(err)
			continue
		}

		if update.Message == nil {
			continue
		}

		// send greeting message for a new user
		if update.Message.NewChatMembers != nil {
			for _, member := range update.Message.NewChatMembers {
				greetingMessage := "Hello, " + member.FirstName + "!\n" +
					"Please register with /register <username> and /team <team>"
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, greetingMessage)
				bot.Send(msg)
			}
			continue
		}

		if !update.Message.IsCommand() {
			waitingCommand, err := getWaitingCommand(svc, update.Message.From.ID, update.Message.Date)
			if err != nil {
				log.Printf("failed to get waiting command: %v\n", err)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Something went wrong. Error: "+err.Error())
				msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
				bot.Send(msg)
				continue
			}
			if waitingCommand == "" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "I don't understand you")
				msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
				bot.Send(msg)
				continue
			}
			switch waitingCommand {
			case "register":
				err := registerUsername(bot, svc, update.Message.From.ID, update.Message.Chat.ID, update.Message.Text)
				if err != nil {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Something went wrong. Error: "+err.Error())
					bot.Send(msg)
				}
			case "team":
				err := registerTeam(bot, svc, update.Message.From.ID, update.Message.Chat.ID, update.Message.Text)
				if err != nil {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Something went wrong. Error: "+err.Error())
					msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
					bot.Send(msg)
				}
			case "code":
				err := sendCode(bot, svc, update.Message.From.ID, update.Message.Chat.ID, update.Message.Text)
				if err != nil {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Something went wrong. Error: "+err.Error())
					bot.Send(msg)
				}
			case "admin":
				err := updateAdmin(bot, svc, update.Message.From.ID, update.Message.Chat.ID, update.Message.Text, true)
				if err != nil {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Something went wrong. Error: "+err.Error())
					bot.Send(msg)
				}
			case "stopadmin":
				err := updateAdmin(bot, svc, update.Message.From.ID, update.Message.Chat.ID, update.Message.Text, false)
				if err != nil {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Something went wrong. Error: "+err.Error())
					bot.Send(msg)
				}
			case "addcode":
				err := addCode(bot, svc, update.Message.From.ID, update.Message.Chat.ID, update.Message.Text)
				if err != nil {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Something went wrong. Error: "+err.Error())
					bot.Send(msg)
				}
			case "removecode":
				err := removeCode(bot, svc, update.Message.From.ID, update.Message.Chat.ID, update.Message.Text)
				if err != nil {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Something went wrong. Error: "+err.Error())
					bot.Send(msg)
				}
			case "a3":
				err := answerPair(bot, svc, update.Message.From.ID, update.Message.Chat.ID, update.Message.Text, "PairA")
				if err != nil {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Something went wrong. Error: "+err.Error())
					bot.Send(msg)
				}
			case "a3answer":
				err := addPair(bot, svc, update.Message.From.ID, update.Message.Chat.ID, update.Message.Text, "PairA")
				if err != nil {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Something went wrong. Error: "+err.Error())
					bot.Send(msg)
				}
			case "b1":
				err := answerPair(bot, svc, update.Message.From.ID, update.Message.Chat.ID, update.Message.Text, "PairB")
				if err != nil {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Something went wrong. Error: "+err.Error())
					bot.Send(msg)
				}
			case "b1answer":
				err := addPair(bot, svc, update.Message.From.ID, update.Message.Chat.ID, update.Message.Text, "PairB")
				if err != nil {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Something went wrong. Error: "+err.Error())
					bot.Send(msg)
				}
			default:
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Wrong behavior. Cannot handle command "+waitingCommand+". Please contact the admin")
				bot.Send(msg)
			}

			continue
		}

		waitingCommand := ""

		switch update.Message.Command() {
		case "start":
			fallthrough
		case "what":
			messageString := "I can help you with the following commands:\n" +
				"/register - set your username\n" +
				"/team - set your team\n" +
				"/code - send the code\n" +
				"/codes - get the codes\n" +
				"/top - get the top\n" +
				"/whoami - get your username and team\n" +
				"/a3 - enter the answer for a3\n" +
				"/b1 - enter the answer for b1\n" +
				"/what - show this message\n"
			if ok, err := isAdmin(svc, update.Message.From.ID); ok && err == nil {
				messageString += "/admin - become an admin\n" +
					"/stopadmin - stop being an admin\n" +
					"/addcode - add a code\n" +
					"/removecode - remove a code\n" +
					"/a3answer - add a a3 answer\n" +
					"/lista3 - list a3\n" +
					"/b1answer - add a b1 answer\n" +
					"/listb1 - list b1\n"
			}
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, messageString)
			bot.Send(msg)
		case "register":
			waitingCommand = "register"
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please provide your username")
			bot.Send(msg)
		case "team":
			waitingCommand = "team"
			teams := [][]tgbotapi.KeyboardButton{
				{
					tgbotapi.NewKeyboardButton("A"),
					tgbotapi.NewKeyboardButton("B"),
				},
				{
					tgbotapi.NewKeyboardButton("C"),
					tgbotapi.NewKeyboardButton("D"),
				},
			}
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please choose your team")
			msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(teams...)
			bot.Send(msg)
		case "a3":
			waitingCommand = "a3"
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please provide the answer")
			bot.Send(msg)
		case "b1":
			waitingCommand = "b1"
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please provide the answer")
			bot.Send(msg)
		case "code":
			waitingCommand = "code"
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please provide the code")
			bot.Send(msg)
		case "codes":
			err := listCodes(bot, svc, update.Message.From.ID, update.Message.Chat.ID)
			if err != nil {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Something went wrong. Error: "+err.Error())
				bot.Send(msg)
			}
		case "top":
			err := listTop(bot, svc, update.Message.From.ID, update.Message.Chat.ID)
			if err != nil {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Something went wrong. Error: "+err.Error())
				bot.Send(msg)
			}
		case "whoami":
			username, err := getUsername(svc, update.Message.From.ID)
			if err == nil {
				if len(username) > 0 {
					team, err := getTeam(svc, update.Message.From.ID)
					if err == nil {
						messageString := ""
						if team != "" {
							messageString = "You are " + username + " from team " + team
						} else {
							messageString = "You are " + username
						}
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, messageString)
						bot.Send(msg)
					} else {
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Something went wrong. Error: "+err.Error())
						bot.Send(msg)
					}
				} else {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "You are not registered")
					bot.Send(msg)
				}
			} else {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Something went wrong. Error: "+err.Error())
				bot.Send(msg)
			}
		// admin commands
		case "addcode":
			waitingCommand = "addcode"
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please provide the code, room and note separated by -")
			bot.Send(msg)
		case "removecode":
			waitingCommand = "removecode"
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please provide the code")
			bot.Send(msg)
		case "admin":
			waitingCommand = "admin"
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please provide the secret")
			bot.Send(msg)
		case "stopadmin":
			waitingCommand = "stopadmin"
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please provide the secret")
			bot.Send(msg)
		case "a3answer":
			waitingCommand = "a3answer"
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please provide the answer")
			bot.Send(msg)
		case "lista3":
			err := listPair(bot, svc, update.Message.From.ID, update.Message.Chat.ID, "PairA")
			if err != nil {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Something went wrong. Error: "+err.Error())
				bot.Send(msg)
			}
		case "b1answer":
			waitingCommand = "b1answer"
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please provide the answer")
			bot.Send(msg)
		case "listb1":
			err := listPair(bot, svc, update.Message.From.ID, update.Message.Chat.ID, "PairB")
			if err != nil {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Something went wrong. Error: "+err.Error())
				bot.Send(msg)
			}
		default:
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "I don't know that command")
			bot.Send(msg)
		}

		if waitingCommand != "" {
			// save the command, fromID and timestamp to DynamoDB
			_, err := svc.PutItem(&dynamodb.PutItemInput{
				TableName: aws.String("WaitingCommand"),
				Item: map[string]*dynamodb.AttributeValue{
					"from_id": {
						N: aws.String(fmt.Sprint(update.Message.From.ID)),
					},
					"command": {
						S: aws.String(waitingCommand),
					},
					"timestamp": {
						N: aws.String(fmt.Sprint(update.Message.Date)),
					},
				},
			})
			if err != nil {
				log.Printf("failed to put item: %v\n", err)
			}
		}
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
