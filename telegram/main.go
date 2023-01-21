package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/PullRequestInc/go-gpt3"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func sendToTelegram(update tgbotapi.Update, response string, bot *tgbotapi.BotAPI) {
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

func main() {
	magacc := []string{}

	b, err := os.ReadFile("ctx.json")
	if err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}
	}
	if len(b) > 0 {
		err = json.Unmarshal(b, &magacc)
		if err != nil {
			panic(err)
		}
	}

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

		logMsg := fmt.Sprintf("\n---\nFrom: %q\nMessage: %s\n", update.Message.From.UserName, update.Message.Text)
		fmt.Printf("logMsg: %q\n", logMsg)

		for len(magacc) > 100 {
			magacc = magacc[1:]
		}
		magacc = append(magacc, logMsg)

		b, err := json.Marshal(magacc)
		if err != nil {
			log.Println(err)
			continue
		}
		err = os.WriteFile("ctx.json", b, 0644)
		if err != nil {
			panic(err)
		}

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

		prompt := string(preContext) + msgContext + "\n---\n" + string(posContext) + logMsg

		log.Println("Prompt: ", prompt)

		maxRetries := 3
	retry:
		err = client.CompletionStreamWithEngine(ctx, gpt3.TextDavinci003Engine, gpt3.CompletionRequest{
			Prompt: []string{
				prompt,
			},
			MaxTokens:   gpt3.IntPtr(2000),
			Temperature: gpt3.Float32Ptr(0.7),
		}, func(resp *gpt3.CompletionResponse) {
			response += resp.Choices[0].Text
		})
		if err != nil {
			log.Println(err)
			// remove os dois primeiros itens do magacc
			magacc = magacc[2:]
			maxRetries--
			if maxRetries == 0 {
				magacc = []string{}
				continue
			}
			goto retry // goto is not evil
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

		sendToTelegram(update, response, bot)
	}
}
