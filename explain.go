package main

import (
	"context"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"gopkg.in/telebot.v3"
	"log"
)

func sendExplainWord(ctx telebot.Context, ai *openai.Client, word string, temperature float32) {
	propmt := fmt.Sprintf("Обьясни значение слова %s", word)
	resp, err := ai.CreateChatCompletion(context.TODO(), openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo1106,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: propmt,
			},
		},
		Temperature: temperature,
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

func anyQuestion(ctx telebot.Context, ai *openai.Client, prompt string) {
	log.Printf("asking gpt: %s", prompt)
	msg, err := ctx.Bot().Reply(ctx.Message(), "ChatGPT: думаю...")
	if err != nil {
		log.Printf("reply error: %v", err)
		return
	}
	resp, err := ai.CreateChatCompletion(context.TODO(), openai.ChatCompletionRequest{
		Model: openai.GPT4TurboPreview,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		//MaxTokens: 400,
	})
	if err != nil {
		log.Printf("openai error: %v", err)
		return
	}
	answer := resp.Choices[0].Message.Content
	log.Printf("gpt answer: %v", answer)
	_, err = ctx.Bot().Edit(msg, fmt.Sprintf("ChatGPT: %s", answer))
	if err != nil {
		log.Printf("edit error: %v", err)
	}
}
