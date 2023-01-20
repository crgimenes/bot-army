package main

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	openai "github.com/openai/api-go"
)

func main() {
	bot, err := tgbotapi.NewBotAPI("YOUR_TELEGRAM_BOT_TOKEN")
	if err != nil {
		log.Panic(err)
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
		client := openai.NewClient("YOUR_OPENAI_API_KEY")
		response, err := client.Completion.Create(openai.CompletionRequest{
			Engine:    "text-davinci-002",
			Prompt:    update.Message.Text,
			MaxTokens: 2048,
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
