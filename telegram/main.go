package main

import (
	"context"
	"log"
	"os"

	"github.com/PullRequestInc/go-gpt3"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func main() {

	apiKey := os.Getenv("GP3_API_KEY")
	if apiKey == "" {
		log.Fatalln("Missing GP3_API_KEY")
	}

	client := gpt3.NewClient(apiKey)
	ctx := context.Background()

	bot, err := tgbotapi.NewBotAPI("TELEGRAM_BOT_TOKEN")
	if err != nil {
		log.Fatalln("Missing TELEGRAM_BOT_TOKEN")
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { // ignore any non-Message Updates
			continue
		}

		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		// Use the OpenAI API to generate a response
		response, err := client.Completion(ctx, gpt3.CompletionRequest{
			Prompt:    []string{update.Message.Text},
			MaxTokens: gpt3.IntPtr(64),
		})

		if err != nil {
			log.Println(err)
			continue
		}

		// Send the response back to the user
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, response.Choices[0].Text)
		bot.Send(msg)
	}
}
