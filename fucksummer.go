package main

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"github.com/sashabaranov/go-openai"
	"gopkg.in/telebot.v3"
	"log"
	"strings"
	"time"
)

func replyFuckSummer(ctx telebot.Context, rediska *redis.Client) {
	const lastSentKey = "fuckSummerSent"
	const sendTimeout = 10 * time.Second

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
}

func checkCalmDownFuckSummer(ctx telebot.Context, rediska *redis.Client, ai *openai.Client) (toCalmDown bool) {
	const userTimeout = 2 * time.Minute
	const userRateWithinTimeout = 5

	userLimitKey := fmt.Sprintf("fuckSummerCount_%d", ctx.Message().Sender.ID)
	incrRes, err := rediska.Incr(context.TODO(), userLimitKey).Result()
	if err == nil {
		log.Printf("%s = %d", userLimitKey, incrRes)
		rediska.Expire(context.TODO(), userLimitKey, userTimeout)
		if incrRes >= userRateWithinTimeout {
			toCalmDown = true
			rediska.Del(context.TODO(), userLimitKey)
		}
	} else {
		log.Printf("incr error: %v", err)
	}
	return
}

func sendCalmDownFuckSummer(ctx telebot.Context, rediska *redis.Client, ai *openai.Client) {
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
