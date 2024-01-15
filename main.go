package main

import (
	"fmt"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/redis/go-redis/v9"
	"github.com/sashabaranov/go-openai"
	tele "gopkg.in/telebot.v3"
	"gopkg.in/telebot.v3/middleware"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
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
		if cfg.Debug {
			log.Printf("is reply=%v", ctx.Message().IsReply())
		}
		lowerText := strings.TrimSpace(strings.ToLower(ctx.Message().Text))

		tempRegex := regexp.MustCompile(`temp (\d\.\d)`)
		tempFound := tempRegex.FindStringSubmatch(lowerText)
		temperature := float32(1.0)
		if tempFound != nil {
			parsedTemp, err := strconv.ParseFloat(tempFound[1], 32)
			if err == nil {
				temperature = float32(parsedTemp)
				log.Printf("temperature set to %f", temperature)
			}
		}

		/*if cfg.Debug || rand.Int()%20 == 0 {
			log.Println("язвим...")
			sendFunnyReply(ctx, ai)
		}*/

		explainWord := []string{
			"обьясни", "объясни",
			"что за слово", "что это",
			"не понял",
			"че", "чё", "что",
		}
		if ctx.Message().IsReply() && stringStartsWith(ctx.Message().Text, explainWord) {
			log.Println("what???")
			replyToText := ctx.Message().ReplyTo.Text
			replyToText = strings.TrimSpace(replyToText)
			if len(replyToText) > 0 && strings.Count(replyToText, " ") == 0 {
				sendExplainWord(ctx, ai, replyToText, temperature)
			}
		}

		postLinkRegex := regexp.MustCompile(`https://pipmy\.ru/.+?\.html`)
		postLinkFound := postLinkRegex.FindString(lowerText)
		if len(postLinkFound) > 0 {
			summary(cfg, ctx, ai, postLinkFound, temperature)
		}

		return nil
	})
}
