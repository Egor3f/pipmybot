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
	"time"
)

type Config struct {
	TelegramToken     string  `env:"TELEGRAM_TOKEN" env-required:"true"`
	OpenAIToken       string  `env:"OPENAI_TOKEN" env-required:"true"`
	OpenAIProxy       string  `env:"OPENAI_PROXY" env-required:"true"`
	ChatsWhitelist    []int64 `env:"CHATS_WHITELIST"`
	Redis             string  `env:"REDIS" env-required:"true"`
	NewPostsChatId    int64   `env:"NEW_POSTS_CHAT_ID" env-required:"true"`
	NewPostsThreadIds []int   `env:"NEW_POSTS_THREAD_IDS" env-required:"true"`
	NewPostsFeedURL   string  `env:"NEW_POSTS_FEED_URL" env-required:"true"`
	ToadBotId         int64   `env:"TOAD_BOT_ID" env-required:"true"`
	NovofonKey        string  `env:"NOVOFON_KEY" env-required:"true"`
	NovofonSecret     string  `env:"NOVOFON_SECRET" env-required:"true"`
	NovofonFrom       string  `env:"NOVOFON_FROM" env-required:"true"`
	Debug             bool    `env:"DEBUG"`
}

func main() {
	cfg := Config{}
	err := cleanenv.ReadEnv(&cfg)
	if err != nil {
		panic(err)
	}
	log.Printf("Config: %+v", cfg)

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

	go func() {
		for {
			log.Println("checking new posts...")
			err := notifyNewPosts(cfg, bot, rediska)
			if err != nil {
				log.Printf("Error notify new posts: %v", err)
			}
			time.Sleep(1 * time.Minute)
		}
	}()

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

		if ctx.Message().Sender.ID == cfg.ToadBotId || cfg.Debug {
			toadCafeSubstr := "новый заказ! на выполнение у вас есть"
			if strings.Contains(lowerText, toadCafeSubstr) {
				notifyToadCafe(cfg, ctx, rediska)
			}
		}

		if ctx.Message().Sender.IsBot {
			return nil
		}

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

		const anyQuestionPrefix = "пип бот"
		if strings.HasPrefix(lowerText, anyQuestionPrefix) {
			prompt, _ := strings.CutPrefix(lowerText, anyQuestionPrefix)
			prompt = strings.TrimSpace(prompt)
			if len(prompt) > 0 {
				anyQuestion(ctx, ai, prompt)
			}
		}

		return nil
	})

	bot.Handle("/toad_notify_start", func(ctx tele.Context) error {
		toggleToadNotufy(ctx, rediska, true)
		return nil
	})
	bot.Handle("/toad_notify_stop", func(ctx tele.Context) error {
		toggleToadNotufy(ctx, rediska, false)
		return nil
	})
	bot.Handle(tele.OnContact, func(ctx tele.Context) error {
		saveToadPhone(ctx, rediska)
		return nil
	})
}
