package main

import (
	"fmt"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/sashabaranov/go-openai"
	tele "gopkg.in/telebot.v3"
	"gopkg.in/telebot.v3/middleware"
	"log"
	"net/http"
	"net/url"
	"slices"
)

type Config struct {
	TelegramToken  string  `env:"TELEGRAM_TOKEN" env-required:"true"`
	OpenAIToken    string  `env:"OPENAI_TOKEN" env-required:"true"`
	OpenAIProxy    string  `env:"OPENAI_PROXY" env-required:"true"`
	ChatsWhitelist []int64 `env:"CHATS_WHITELIST"`
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
	setupMiddlewares(cfg, bot)
	setupHandlers(cfg, bot, ai)
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

func setupHandlers(cfg Config, bot *tele.Bot, ai *openai.Client) {
	bot.Handle(tele.OnSticker, func(ctx tele.Context) error {
		const fuckSummer = "AgADMToAAklRsEg"
		if ctx.Message().Sticker.UniqueID == fuckSummer {
			log.Println("нахуй лету")
			return ctx.Reply(ctx.Message().Sticker)
		}
		return nil
	})
}
