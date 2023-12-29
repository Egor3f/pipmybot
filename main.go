package main

import (
	"context"
	"fmt"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/redis/go-redis/v9"
	"github.com/sashabaranov/go-openai"
	tele "gopkg.in/telebot.v3"
	"gopkg.in/telebot.v3/middleware"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"
)

type Config struct {
	TelegramToken  string  `env:"TELEGRAM_TOKEN" env-required:"true"`
	OpenAIToken    string  `env:"OPENAI_TOKEN" env-required:"true"`
	OpenAIProxy    string  `env:"OPENAI_PROXY" env-required:"true"`
	ChatsWhitelist []int64 `env:"CHATS_WHITELIST"`
	Redis          string  `env:"REDIS" env-required:"true"`
	Debug          bool    `env:"DEBUG"`
}

func main() {
	cfg := Config{}
	err := cleanenv.ReadEnv(&cfg)
	if err != nil {
		panic(err)
	}

	bot, err := tele.NewBot(tele.Settings{
		Token: cfg.TelegramToken,
	})
	if err != nil {
		panic(err)
	}

	aiConfig := openai.DefaultConfig(cfg.OpenAIToken)
	proxyUrl, err := url.Parse(cfg.OpenAIProxy)
	if err != nil {
		panic(err)
	}
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyUrl),
	}
	aiConfig.HTTPClient = &http.Client{
		Transport: transport,
	}
	ai := openai.NewClientWithConfig(aiConfig)

	rediska := redis.NewClient(&redis.Options{Addr: cfg.Redis})

	setupMiddlewares(cfg, bot)
	setupHandlers(cfg, bot, ai, rediska)
	bot.Start()
}

func setupMiddlewares(cfg Config, bot *tele.Bot) {
	bot.Use(restrictChats(cfg.ChatsWhitelist))
	bot.Use(middleware.Logger())
}

func restrictChats(whitelist []int64) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(ctx tele.Context) error {
			if slices.Contains(whitelist, ctx.Chat().ID) {
				return next(ctx)
			}
			return fmt.Errorf("chat id %d not in whitelist", ctx.Chat().ID)
		}
	}
}

func setupHandlers(cfg Config, bot *tele.Bot, ai *openai.Client, rediska *redis.Client) {
	bot.Handle(tele.OnSticker, func(ctx tele.Context) error {
		const fuckSummer = "AgADMToAAklRsEg"

		if ctx.Message().Sticker.UniqueID == fuckSummer {
			log.Println("нахуй лету")
			replyFuckSummer(ctx, rediska)
			if checkCalmDownFuckSummer(ctx, rediska, ai) {
				sendCalmDownFuckSummer(ctx, rediska, ai)
			}
			return nil
		}
		return nil
	})

	bot.Handle(tele.OnText, func(ctx tele.Context) error {
		if cfg.Debug || rand.Int()%20 == 0 {
			log.Println("язвим...")
			sendFunnyReply(ctx, ai)
		}

		explainWord := []string{
			"обьясни", "объясни",
			"что за слово", "что это",
			"не понял",
			"че", "чё",
		}
		if ctx.Message().ReplyTo != nil && stringContains(ctx.Message().Text, explainWord) {
			log.Println("what???")
			replyToText := ctx.Message().ReplyTo.Text
			replyToText = strings.TrimSpace(replyToText)
			if strings.Count(replyToText, " ") == 0 {
				sendExplainWord(ctx, ai, replyToText)
			}
		}
		return nil
	})
}

func sendExplainWord(ctx tele.Context, ai *openai.Client, word string) {
	propmt := fmt.Sprintf("Обьясни значение слова %s", word)
	resp, err := ai.CreateChatCompletion(context.TODO(), openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo1106,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: propmt,
			},
		},
	})
	if err != nil {
		log.Printf("openai error: %v", err)
		return
	}
	answer := resp.Choices[0].Message.Content
	log.Printf("gpt answer: %v", answer)
	err = ctx.Reply(answer)
	if err != nil {
		log.Printf("reply error: %v", err)
	}
}

func sendFunnyReply(ctx tele.Context, ai *openai.Client) {
	text := ctx.Message().Text
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) < 10 || utf8.RuneCountInString(text) > 300 {
		return
	}
	const preprompt = "Ты шутливый чат бот, добавленный в группу. Ты должен язвительно комментировать сообщения. Отвечай коротко"
	model := openai.GPT3Dot5Turbo1106
	if rand.Int()%2 == 0 {
		model = openai.GPT4TurboPreview
	}
	resp, err := ai.CreateChatCompletion(context.TODO(), openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: preprompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: text,
			},
		},
	})
	if err != nil {
		log.Printf("openai error: %v", err)
		return
	}
	answer := resp.Choices[0].Message.Content
	log.Printf("gpt answer: %v", answer)
	err = ctx.Reply(answer)
	if err != nil {
		log.Printf("reply error: %v", err)
	}
}
