package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/PullRequestInc/go-gpt3"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	magacc := []string{}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatalln("Missing OPENAI_API_KEY")
	}

	telegramBotToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if telegramBotToken == "" {
		log.Fatalln("Missing TELEGRAM_BOT_TOKEN")
	}

	client := gpt3.NewClient(apiKey)
	ctx := context.Background()

	bot, err := tgbotapi.NewBotAPI(telegramBotToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { // ignore any non-Message Updates
			continue
		}

		logMsg := fmt.Sprintf("\n---------------\nFrom: %q\nTo: %q\nMessage: %s\n", update.Message.From.UserName, update.Message.Chat.UserName, update.Message.Text)
		fmt.Printf("logMsg: %q\n", logMsg)

		if len(magacc) > 100 {
			magacc = magacc[1:]
		}
		magacc = append(magacc, logMsg)

		//if update.Message.Chat.UserName != bot.Self.UserName &&
		//	!strings.Contains(update.Message.Text, bot.Self.UserName) {
		//	continue
		//}

		msgContext := ""
		for _, v := range magacc {
			msgContext += v
		}

		response := ""
		preContext, err := os.ReadFile("pre_ctx.txt")
		if err != nil {
			log.Println(err)
			continue
		}

		posContext, err := os.ReadFile("pos_ctx.txt")
		if err != nil {
			log.Println(err)
			continue
		}

		prompt := string(preContext) + msgContext + "\n---------------\n" + string(posContext) + logMsg

		log.Println("Prompt: ", prompt)

		err = client.CompletionStreamWithEngine(ctx, gpt3.TextDavinci003Engine, gpt3.CompletionRequest{
			Prompt: []string{
				prompt,
			},
			MaxTokens:   gpt3.IntPtr(2000),
			Temperature: gpt3.Float32Ptr(1.1),
		}, func(resp *gpt3.CompletionResponse) {
			response += resp.Choices[0].Text
		})
		if err != nil {
			log.Println(err)
			continue
		}

		if strings.Contains(response, "++++") {
			log.Printf("msg ignored: %s", response)
			continue
		}

		if response == "" {
			log.Println("empty response")
			continue
		}

		magacc = append(magacc, response)

		msg := tgbotapi.Chattable(&tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:           update.Message.Chat.ID,
				ReplyToMessageID: update.Message.MessageID,
			},
			Text:                  response,
			ParseMode:             "markdown",
			DisableWebPagePreview: true,
		})

		bot.Send(msg)
	}
}
