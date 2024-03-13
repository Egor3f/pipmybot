package main

import (
	"fmt"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/redis/go-redis/v9"
	"github.com/sashabaranov/go-openai"
	tele "gopkg.in/telebot.v3"
	"log"
	"net/http"
	"net/url"
	"os"
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
	TelegramClient    struct {
		AppId   int    `env:"TELEGRAM_APP_ID" env-required:"true"`
		AppHash string `env:"TELEGRAM_APP_HASH" env-required:"true"`
	}
	AdminIds        []int64 `env:"ADMIN_IDS" env-required:"true"`
	ChannelUsername string  `env:"CHANNEL_USERNAME" env-required:"true"`
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

	if !cfg.Debug {
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
	}

	if _, err := os.Stat("session.json"); !os.IsNotExist(err) {
		log.Printf("session.json found; starting userbot...")
		startUserBot(cfg, rediska)
	} else {
		log.Printf("session.json not found; skipping userbot...")
	}

	bot.Start()
}

func startUserBot(cfg Config, rediska *redis.Client) {
	go func() {
		monitorBotMessages(cfg, func(client *telegram.Client, update *tg.UpdateNewChannelMessage) {
			msg, ok := update.Message.(*tg.Message)
			if !ok {
				return
			}
			peer, ok := msg.FromID.(*tg.PeerUser)
			if !ok {
				return
			}
			//log.Printf("Peer: %+v", peer)
			//log.Printf("Mesg: %+v", msg)
			if peer.UserID == cfg.ToadBotId {
				log.Printf("Toad bot message: %s", msg.Message)
				log.Printf("Toad bot entities: %s", msg.Entities)
				if !strings.Contains(msg.Message, "новый заказ") || len(msg.Entities) == 0 {
					return
				}
				mention, ok := msg.Entities[0].(*tg.MessageEntityMentionName)
				if !ok {
					return
				}
				log.Printf("Toad cafe: user mentioned: %d", mention.UserID)
				notifyToadCafe(cfg, rediska, mention.UserID)
			} else {
				checkAndRemoveSpam(cfg, client, msg, rediska)
			}
		})
	}()
}

func setupMiddlewares(cfg Config, bot *tele.Bot) {
	//bot.Use(restrictChats(cfg.ChatsWhitelist))
	//if cfg.Debug {
	//	bot.Use(middleware.Logger())
	//} else {
	bot.Use(liteLogger)
	//}
}

func liteLogger(next tele.HandlerFunc) tele.HandlerFunc {
	return func(ctx tele.Context) error {
		log.Printf("Received bot message: User=%d Chat=%d Msg=%s", ctx.Message().Sender.ID, ctx.Message().Chat.ID, ctx.Message().Text)
		return next(ctx)
	}
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
			// replyFuckSummer(ctx, rediska)
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

	handleWelcome(bot)
	handleAntispam(bot, cfg, rediska)
}
