package main

import (
	"context"
	"fmt"
	"github.com/gocolly/colly/v2"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/redis/go-redis/v9"
	"github.com/sashabaranov/go-openai"
	tele "gopkg.in/telebot.v3"
	"gopkg.in/telebot.v3/middleware"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
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
		if cfg.Debug {
			log.Printf("is reply=%v", ctx.Message().IsReply())
		}
		lowerText := strings.TrimSpace(strings.ToLower(ctx.Message().Text))

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
				sendExplainWord(ctx, ai, replyToText)
			}
		}

		postLinkRegex := regexp.MustCompile(`https://pipmy\.ru/.+?\.html`)
		postLinkFound := postLinkRegex.FindString(lowerText)
		if len(postLinkFound) > 0 {
			summary(cfg, ctx, ai, postLinkFound)
		}

		return nil
	})
}

func summary(cfg Config, ctx tele.Context, ai *openai.Client, postLink string) {
	title := fmt.Sprintf("ChatGPT: Краткое содержание поста %s\n\n", postLink)
	msg, err := ctx.Bot().Reply(ctx.Message(), fmt.Sprintf("%sЗагружаем пост... ", title), tele.NoPreview)
	if err != nil {
		log.Printf("reply error: %v", err)
		return
	}

	postContent, err := parsePost(cfg, postLink)
	if err != nil {
		log.Printf("error parsing post: %v", err)
		return
	}

	if len(postContent) == 0 {
		log.Println("post length=0")
		_, err = ctx.Bot().Edit(
			msg,
			fmt.Sprintf("%sПост содержит только картинки или пустой. Краткое содержание недоступно", title),
			tele.NoPreview,
		)
		if err != nil {
			log.Printf("edit error: %v", err)
		}
		return
	}

	msg, err = ctx.Bot().Edit(msg, fmt.Sprintf("%sПост загружен, генерируем описание через GPT...", title), tele.NoPreview)
	if err != nil {
		log.Printf("edit error: %v", err)
		return
	}

	sentenciesEstimate := float64(utf8.RuneCountInString(postContent)) / 300 // magic hardcoded constant, yep, shitcode
	log.Printf("len=%d, estimate=%f", utf8.RuneCountInString(postContent), sentenciesEstimate)
	propmt := fmt.Sprintf(
		"Я пришлю тебе пост, а тебе нужно будет составить краткое содержание. Длина %d-%d предложений. "+
			"\n%s", int(math.Floor(sentenciesEstimate)), int(math.Ceil(sentenciesEstimate)+1), postContent)
	resp, err := ai.CreateChatCompletion(context.TODO(), openai.ChatCompletionRequest{
		Model: openai.GPT4TurboPreview,
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

	_, err = ctx.Bot().Edit(msg, fmt.Sprintf("%s%s", title, answer), tele.NoPreview)
	if err != nil {
		log.Printf("edit error: %v", err)
		return
	}
}

func parsePost(cfg Config, link string) (result string, err error) {
	col := colly.NewCollector()
	col.Limit(&colly.LimitRule{
		Delay:       1 * time.Second,
		Parallelism: 1,
	})
	col.OnError(func(resp *colly.Response, err2 error) {
		err = fmt.Errorf("scraper error: %w", err2)
	})
	col.OnHTML("div.item-content", func(e *colly.HTMLElement) {
		result = e.Text
		if cfg.Debug {
			log.Println(result)
		}
		result = strings.ReplaceAll(result, "replaceImgClass()", "")
		result = strings.TrimSpace(result)
		/*
			p := post{}
			p.text = e.Text
			e.ForEach("img[src]", func(i int, e *colly.HTMLElement) {
				img := postImage{url: e.Attr("src")}
				if !strings.HasPrefix(img.url, "http") {
					img.url = "https:// " + e.Request.URL.Host + img.url
				}
				p.images = append(p.images, img)
			})
			posts = append(posts, p)
		*/
	})
	col.Visit(link)
	col.Wait()
	return
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
