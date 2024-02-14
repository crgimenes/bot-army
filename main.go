package main

import (
	"botarmy/database"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	openai "github.com/sashabaranov/go-openai"
)

var (
	openaiAPIKey     string
	telegramBotToken string
	systemContext    []byte
	contextQuery     []string
	mx               sync.Mutex
	db               *database.Database
)

func getOpenAI(userQuery []string) (string, error) {
	c := openai.NewClient(openaiAPIKey)
	ctx := context.Background()

	context := ""
	for _, v := range userQuery[:len(userQuery)-1] {
		context += v + "\n\n"
	}

	systemQuery := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleSystem,
		Content: fmt.Sprintf("%s\nmensagens de contexto:%s ",
			systemContext, // pre-existing context
			context),      // user context
	}

	message := []openai.ChatCompletionMessage{
		systemQuery,
	}

	message = append(message, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: userQuery[len(userQuery)-1], // last message is the user query
	})

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

func updateContext(context string) {
	mx.Lock()
	defer mx.Unlock()
	contextQuery = append(contextQuery, context)
	if len(contextQuery) > 5 {
		contextQuery = contextQuery[1:]
	}
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	from := update.Message.From
	logMsg := fmt.Sprintf("\n---\nFrom: %q\nMessage: %s\n", from.Username, update.Message.Text)

	q := contextQuery
	q = append(q, string(logMsg))

	r, err := getOpenAI(q)
	if err != nil {
		log.Printf("Error getting OpenAI response: %v", err)
		return
	}

	log.Printf("Query: %s\nResponse: %s", logMsg, r)

	if r == "++++" {
		log.Println("Empty response")
		err = db.AddMessage("non_query", logMsg, "")
		if err != nil {
			log.Printf("Error adding message to database: %v", err)
		}
		return
	}

	err = db.AddMessage("query", logMsg, r)
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

	updateContext(logMsg)
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

	systemContext, err = os.ReadFile("ctx.txt")
	if err != nil {
		if !os.IsNotExist(err) {
			log.Fatalf("Error reading pre_ctx.txt: %v", err)
		}
		log.Println("ctx.txt not found")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	opts := []bot.Option{
		bot.WithDefaultHandler(handler),
	}

	b, err := bot.New(telegramBotToken, opts...)
	if err != nil {
		log.Fatalf("Error creating bot: %v", err)
	}

	log.Println("Bot started")

	b.Start(ctx)
}
