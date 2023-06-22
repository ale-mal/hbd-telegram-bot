package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

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

	for _, record := range kinesisEvent.Records {
		var update tgbotapi.Update
		if err := json.Unmarshal([]byte(record.Kinesis.Data), &update); err != nil {
			log.Println(err)
			continue
		}

		if _, err := bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)); err != nil {
			log.Println(err)
			continue
		}
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
