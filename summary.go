package main

import (
	"context"
	"fmt"
	"github.com/gocolly/colly/v2"
	"github.com/sashabaranov/go-openai"
	"gopkg.in/telebot.v3"
	"log"
	"strings"
	"time"
)

type partType int8

const (
	PartTypeText  partType = 1
	PartTypeImage partType = 2
)

type postPart struct {
	pType   partType
	content string
}

func summary(cfg Config, ctx telebot.Context, ai *openai.Client, postLink string, temperature float32) {
	title := fmt.Sprintf("ChatGPT: Краткое содержание поста %s\n\n", postLink)
	msg, err := ctx.Bot().Reply(ctx.Message(), fmt.Sprintf("%sЗагружаем пост... ", title), telebot.NoPreview)
	if err != nil {
		log.Printf("reply error: %v", err)
		return
	}

	postContent, err := parsePost(cfg, ai, postLink)
	if err != nil {
		log.Printf("error parsing post: %v", err)
		return
	}

	if len(postContent) == 0 {
		log.Println("post length=0")
		_, err = ctx.Bot().Edit(
			msg,
			fmt.Sprintf("%s Ошибка. Краткое содержание недоступно", title),
			telebot.NoPreview,
		)
		if err != nil {
			log.Printf("edit error: %v", err)
		}
		return
	}

	msg, err = ctx.Bot().Edit(msg, fmt.Sprintf("%sПост загружен, генерируем описание через GPT...", title), telebot.NoPreview)
	if err != nil {
		log.Printf("edit error: %v", err)
		return
	}

	hasImages := false
	multiContent := []openai.ChatMessagePart{
		{
			Type: openai.ChatMessagePartTypeText,
			Text: "Я пришлю тебе пост, а тебе нужно будет составить краткое содержание. Длина 3-5 предложений.\n",
		},
	}
	for _, part := range postContent {
		if part.pType == PartTypeImage {
			hasImages = true
			multiContent = append(multiContent, openai.ChatMessagePart{
				Type:     openai.ChatMessagePartTypeImageURL,
				ImageURL: &openai.ChatMessageImageURL{URL: part.content, Detail: openai.ImageURLDetailHigh},
			})
		} else {
			multiContent = append(multiContent, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeText,
				Text: part.content,
			})
		}
	}

	model := openai.GPT4TurboPreview
	if hasImages {
		model = openai.GPT4VisionPreview
	}
	resp, err := ai.CreateChatCompletion(context.TODO(), openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:         openai.ChatMessageRoleUser,
				MultiContent: multiContent,
			},
		},
		MaxTokens:   500,
		Temperature: temperature,
	})
	if err != nil {
		log.Printf("openai error: %v", err)
		return
	}
	log.Println(resp)
	answer := resp.Choices[0].Message.Content
	log.Printf("gpt answer: %v", answer)

	_, err = ctx.Bot().Edit(msg, fmt.Sprintf("%s%s", title, answer), telebot.NoPreview)
	if err != nil {
		log.Printf("edit error: %v", err)
		return
	}
}

func sanitizeContent(s string) string {
	s = strings.ReplaceAll(s, "replaceImgClass()", "")
	s = strings.TrimSpace(s)
	return s
}

func parsePost(cfg Config, ai *openai.Client, link string) (result []postPart, err error) {
	col := colly.NewCollector()
	col.Limit(&colly.LimitRule{
		Delay:       1 * time.Second,
		Parallelism: 1,
	})
	col.OnError(func(resp *colly.Response, err2 error) {
		err = fmt.Errorf("scraper error: %w", err2)
	})
	col.OnHTML("div.item-content", func(e *colly.HTMLElement) {
		e.ForEach("*", func(i int, innerElem *colly.HTMLElement) {
			childrenCount := innerElem.DOM.Children().Length()
			log.Printf("%d: found element: %v, children count=%d", i, innerElem.Name, childrenCount)
			if childrenCount > 0 {
				return
			}

			if innerElem.Name == "img" {
				imgUrl := innerElem.Attr("src")
				if !strings.HasPrefix(imgUrl, "http") {
					imgUrl = "https://" + e.Request.URL.Host + imgUrl
				}
				result = append(result, postPart{
					pType:   PartTypeImage,
					content: imgUrl,
				})
			} else {
				text := sanitizeContent(innerElem.Text)
				if len(text) > 0 {
					result = append(result, postPart{
						pType:   PartTypeText,
						content: text,
					})
				}
			}
		})

		if cfg.Debug {
			log.Println(result)
		}
	})
	col.Visit(link)
	col.Wait()
	return
}
