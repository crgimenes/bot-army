package main

import (
	"botarmy/database"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	openai "github.com/sashabaranov/go-openai"
)

var (
	openaiAPIKey     string
	telegramBotToken string
	systemContext    string
	mx               sync.Mutex
	db               *database.Database
	bannedUsers      map[string]struct{}
	help             string
)

func getOpenAI(userQuery string) (string, error) {
	c := openai.NewClient(openaiAPIKey)
	ctx := context.Background()

	systemQuery := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: systemContext,
	}

	message := []openai.ChatCompletionMessage{
		systemQuery,
		{
			Role:    openai.ChatMessageRoleUser,
			Content: userQuery,
		},
	}

	req := openai.ChatCompletionRequest{
		Model:     openai.GPT3Dot5Turbo,
		MaxTokens: 2000,
		Messages:  message,
		Stream:    true,
	}
	stream, err := c.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	r := ""
	for {
		response, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return r, nil
			}
			return "", err
		}
		r += response.Choices[0].Delta.Content
	}
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil ||
		update.Message.Text == "" ||
		update.Message.From == nil {
		return
	}

	from := update.Message.From

	_, ok := bannedUsers[from.Username]
	if ok {
		return
	}

	msg := update.Message.Text
	if msg == "/help" {
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ParseMode: "Markdown",
			ChatID:    update.Message.Chat.ID,
			Text:      help,
		})
		if err != nil {
			log.Printf("Error sending message: %v", err)
		}
		return
	}

	if strings.HasPrefix(msg, "/ask ") {
		msg = strings.TrimPrefix(msg, "/ask ")
		msg = fmt.Sprintf("\nFrom: %q\nAsk: %s\n", from.Username, update.Message.Text)

		r, err := getOpenAI(msg)
		if err != nil {
			log.Printf("Error getting OpenAI response: %v", err)
			return
		}

		log.Printf("Query: %s\nResponse: %s", msg, r)

		err = db.AddMessage("query", msg, r)
		if err != nil {
			log.Printf("Error adding message to database: %v", err)
		}

		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ParseMode: "Markdown",
			ChatID:    update.Message.Chat.ID,
			Text:      r,
		})
		if err != nil {
			log.Printf("Error sending message: %v", err)
		}
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	var err error
	openaiAPIKey = os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		log.Println("OPENAI_API_KEY environment variable is not set")
		return
	}

	telegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	if telegramBotToken == "" {
		log.Println("TELEGRAM_BOT_TOKEN environment variable is not set")
		return
	}

	db, err = database.New()
	if err != nil {
		log.Fatalf("Error creating database: %v", err)
	}

	systemContextAux, err := os.ReadFile("ctx.txt")
	if err != nil {
		if !os.IsNotExist(err) {
			log.Fatalf("Error reading ctx.txt: %v", err)
		}
		log.Println("ctx.txt not found")
	}
	systemContext = string(systemContextAux)

	helpAux, err := os.ReadFile("help.md")
	if err != nil {
		if !os.IsNotExist(err) {
			log.Fatalf("Error reading help.md: %v", err)
		}
		log.Println("help.md not found")
	}
	help = string(helpAux)

	bannedUsersAux, err := os.ReadFile("banned_users.txt")
	if err != nil {
		if !os.IsNotExist(err) {
			log.Fatalf("Error reading banned_users.txt: %v", err)
		}
		log.Println("banned_users.txt not found")
	}

	bannedUsers = make(map[string]struct{})
	sba := strings.Split(string(bannedUsersAux), "\n")
	for _, v := range sba {
		if v == "" {
			continue
		}
		if v[0] == '#' {
			continue
		}
		splitComment := strings.Split(v, "#")
		if len(splitComment) > 1 {
			v = splitComment[0]
		}
		v = strings.TrimSpace(v)
		bannedUsers[v] = struct{}{}
	}

	/////////////////////////////////////////
	opts := []bot.Option{
		bot.WithDefaultHandler(handler),
	}

	b, err := bot.New(telegramBotToken, opts...)
	if err != nil {
		log.Fatalf("Error creating bot: %v", err)
	}

	log.Println("Bot started")

	b.Start(context.Background())
}
