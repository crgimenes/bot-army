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

var (
	maxTokens = 3000
)

func createPrompt(magacc []string, logMsg string) string {
	msgContext := ""
	for _, v := range magacc {
		msgContext += v
	}

	preContext, err := os.ReadFile("pre_ctx.txt")
	if err != nil {
		if !os.IsNotExist(err) {
			log.Println(err)
		}
	}

	posContext, err := os.ReadFile("pos_ctx.txt")
	if err != nil {
		if !os.IsNotExist(err) {
			log.Println(err)
		}
	}

	prompt := string(preContext) + msgContext + "\n---\n" + string(posContext) + logMsg
	return prompt
}

func loadContext() []string {
	magacc := []string{}

	b, err := os.ReadFile("ctx.json")
	if err != nil {
		if !os.IsNotExist(err) {
			log.Fatalln(err)
		}
	}
	if len(b) > 0 {
		err = json.Unmarshal(b, &magacc)
		if err != nil {
			log.Fatalln(err)
		}
	}
	return magacc
}

func saveContext(magacc []string) {
	b, err := json.Marshal(magacc)
	if err != nil {
		log.Fatalln(err)
	}

	err = os.WriteFile("ctx.json", b, 0644)
	if err != nil {
		log.Fatalln(err)
	}
}

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
	chatMsgs := loadContext()

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

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { // ignore any non-Message Updates
			continue
		}

		logMsg := fmt.Sprintf("\n---\nFrom: %q\nMessage: %s\n", update.Message.From.UserName, update.Message.Text)

		fmt.Printf("Received message: %s\n", update.Message.Text)

		if len(chatMsgs) > 5 {
			chatMsgs = chatMsgs[len(chatMsgs)-5:]
		}

		//if update.Message.Chat.UserName != bot.Self.UserName &&
		//	!strings.Contains(update.Message.Text, bot.Self.UserName) {
		//	continue
		//}

		buf := strings.Builder{}

		prompt := createPrompt(chatMsgs, logMsg)
		maxRetries := 3 // TODO: make this configurable
		retries := maxRetries
	retry:
		buf.Reset()
		err = client.CompletionStreamWithEngine(ctx, gpt3.TextDavinci003Engine, gpt3.CompletionRequest{
			Prompt: []string{
				prompt,
			},
			MaxTokens:   gpt3.IntPtr(maxTokens),
			Temperature: gpt3.Float32Ptr(0.5), // TODO: make this configurable
		}, func(resp *gpt3.CompletionResponse) {
			buf.WriteString(resp.Choices[0].Text)
		})
		if err != nil {
			log.Printf("GPT-3 error: %s, retrying n: %d", err, maxRetries-retries+1)
			if len(chatMsgs) > 1 {
				chatMsgs = chatMsgs[:len(chatMsgs)-1]
			}
			retries--
			if retries <= 0 {
				log.Printf("GPT-3 error: %s, max retries reached", err)
				// clear
				chatMsgs = []string{}
				buf.Reset()
				continue
			}
			goto retry // goto is not evil
		}

		response := buf.String()

		if strings.Contains(response, "++++") {
			log.Printf("log: %q msg ignored: %q\n", logMsg, response)
			continue
		}

		if response == "" {
			log.Println("empty response")
			continue
		}

		log.Printf("msg: %s", response)

		chatMsgs = append(chatMsgs, logMsg, response)
		saveContext(chatMsgs)
		sendToTelegram(update, response, bot)
	}
}
