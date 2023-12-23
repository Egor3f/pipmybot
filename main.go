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
	"strings"
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

		const userTimeout = 2 * time.Minute
		const userRateWithinTimeout = 5

		if ctx.Message().Sticker.UniqueID == fuckSummer {
			log.Println("нахуй лету")
			if cmdRes := rediska.Exists(context.TODO(), lastSentKey); cmdRes.Err() == nil && cmdRes.Val() == 1 {
				log.Println("уже слали лету нахуй. ждем...")
			} else {
				err := ctx.Reply(ctx.Message().Sticker)
				if err == nil {
					cmdRes := rediska.SetEx(context.TODO(), lastSentKey, ".", sendTimeout)
					if cmdRes.Err() != nil {
						log.Printf("setex error: %v", cmdRes.Err())
					}
				}
			}

			userLimitKey := fmt.Sprintf("fuckSummerCount_%d", ctx.Message().Sender.ID)
			incrRes, err := rediska.Incr(context.TODO(), userLimitKey).Result()
			if err == nil {
				log.Printf("%s = %d", userLimitKey, incrRes)
				rediska.Expire(context.TODO(), userLimitKey, userTimeout)
				if incrRes >= userRateWithinTimeout {
					calmDownFuckSummer(ctx, ai, rediska)
					rediska.Del(context.TODO(), userLimitKey)
				}
			} else {
				log.Printf("incr error: %v", err)
			}
			return err
		}
		return nil
	})
}

func calmDownFuckSummer(ctx tele.Context, ai *openai.Client, rediska *redis.Client) {
	const calmDownCache = "calmDownFuckSummerCache"
	const namePlaceholder = "Джон"
	calmDownMessage, err := rediska.Get(context.TODO(), calmDownCache).Result()
	if err == nil {
		calmDownMessage = strings.ReplaceAll(calmDownMessage, namePlaceholder, ctx.Message().Sender.FirstName+" "+ctx.Message().Sender.LastName)
		err = ctx.Reply(calmDownMessage)
		if err != nil {
			log.Printf("calm down error: %v", err)
		}
	} else {
		log.Println("calm down is not ready yet")
	}

	// Сгенерировать следующий ответ и записать в кеш. Джон использвается как замена, т.к. мы не знаем, кто следующим схулиганит
	go func() {
		calmDownRequest := fmt.Sprintf(`
Ты бот, состоящий в груповом чате. 
У нас принято посылать нахуй другого пользователя под именем "Лето" словами "нахуй лету". 
Это нормально, и он радуется, если делать это изредка.
Но один из пользователей под именем "%s" делает это слишком часто. Придумай ответ для него, чтобы его успокоить.
Отвечай коротко, в язвительной форме.
`, namePlaceholder)
		resp, err := ai.CreateChatCompletion(context.TODO(), openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo1106,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: calmDownRequest,
				},
			},
		})
		if err == nil {
			answer := resp.Choices[0].Message.Content
			log.Printf("gpt answer: %v", answer)
			rediska.Set(context.TODO(), calmDownCache, answer, 0)
		} else {
			log.Printf("gpt error: %v", err)
		}
	}()
}
