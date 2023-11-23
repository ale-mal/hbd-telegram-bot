package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

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
			Description: "set the username",
		},
		{
			Command:     "team",
			Description: "set the team",
		},
		{
			Command:     "code",
			Description: "send the code",
		},
		{
			Command:     "codes",
			Description: "get the codes",
		},
	}
	if _, err := bot.Send(tgbotapi.NewSetMyCommands(commands...)); err != nil {
		log.Printf("failed to set commands: %v\n", err)
		return err
	}
	return nil
}

func registerUsername(bot *tgbotapi.BotAPI, svc *dynamodb.DynamoDB, chatID int64, username string) error {
	if username == "" {
		msg := tgbotapi.NewMessage(chatID, "Please provide a username")
		bot.Send(msg)
		return nil
	}

	// check if the username with this chatID already exists
	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String("users"),
		Key: map[string]*dynamodb.AttributeValue{
			"chatID": {
				N: aws.String(fmt.Sprint(chatID)),
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
			TableName: aws.String("users"),
			Key: map[string]*dynamodb.AttributeValue{
				"chatID": {
					N: aws.String(fmt.Sprint(chatID)),
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
			TableName: aws.String("users"),
			Item: map[string]*dynamodb.AttributeValue{
				"chatID": {
					N: aws.String(fmt.Sprint(chatID)),
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

		if !update.Message.IsCommand() {
			continue
		}

		switch update.Message.Command() {
		case "register":
			registerUsername(bot, svc, update.Message.Chat.ID, update.Message.CommandArguments())
		case "team", "code", "codes":
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Not implemented yet")
			bot.Send(msg)
		case "admin":
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "I see you are an admin! Your argument is '"+update.Message.CommandArguments()+"'")
			bot.Send(msg)
		}
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
