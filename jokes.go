package main

import (
	"context"
	"github.com/sashabaranov/go-openai"
	"gopkg.in/telebot.v3"
	"log"
	"math/rand"
	"strings"
	"unicode/utf8"
)

func sendFunnyReply(ctx telebot.Context, ai *openai.Client) {
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
