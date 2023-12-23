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
	"net/http"
	"net/url"
	"slices"
	"time"
)

type Config struct {
	TelegramToken  string  `env:"TELEGRAM_TOKEN" env-required:"true"`
	OpenAIToken    string  `env:"OPENAI_TOKEN" env-required:"true"`
	OpenAIProxy    string  `env:"OPENAI_PROXY" env-required:"true"`
	ChatsWhitelist []int64 `env:"CHATS_WHITELIST"`
	Redis          string  `env:"REDIS" env-required:"true"`
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
		const lastSentKey = "fuckSummerSent"
		const sendTimeout = 10 * time.Second
		if ctx.Message().Sticker.UniqueID == fuckSummer {
			log.Println("нахуй лету")
			if cmdRes := rediska.Exists(context.TODO(), lastSentKey); cmdRes.Err() == nil && cmdRes.Val() == 1 {
				log.Println("уже слали лету нахуй. ждем...")
				return nil
			}
			err := ctx.Reply(ctx.Message().Sticker)
			if err == nil {
				cmdRes := rediska.SetEx(context.TODO(), lastSentKey, ".", sendTimeout)
				if cmdRes.Err() != nil {
					log.Printf("%v", cmdRes.Err())
				}
			}
			return err
		}
		return nil
	})
}
