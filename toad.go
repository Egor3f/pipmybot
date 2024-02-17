package main

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"gopkg.in/telebot.v3"
	"io"
	"log"
	"net/http"
	"regexp"
)

func toggleToadNotufy(ctx telebot.Context, rediska *redis.Client, isEnabled bool) {
	err := saveStateToadNotify(ctx, rediska, isEnabled)
	if err != nil {
		log.Printf("Toggle noad notify error: %v", err)
		return
	}

	if isEnabled {
		phone, err := getToadPhone(rediska, ctx.Message().Sender.ID)
		if err != nil {
			log.Printf("Get toad phone error: %v", err)
			return
		}
		if len(phone) == 0 {
			askToadPhone(ctx)
		} else {
			welcomeToadNotify(ctx, rediska)
		}
	} else {
		err = ctx.Reply("Уведомления выключены", telebot.RemoveKeyboard)
		if err != nil {
			log.Printf("Toad turned off reply error: %v", err)
			return
		}
	}
}

func askToadPhone(ctx telebot.Context) {
	menu := telebot.ReplyMarkup{}
	phoneButton := menu.Contact("Отправить боту номер телефона")
	menu.Reply(menu.Row(phoneButton))
	err := ctx.Reply(
		"Уведомления о работе в кафетерии включатся, когда бот узнает ваш номер телефона. "+
			"Для этого нажмите кнопку под полем ввода!!! "+
			"Если вы беспокоитесь о спаме - не переживайте, расслабьтесь, мы об этом позаботимся сами.", &menu)
	if err != nil {
		log.Printf("Phone menu reply error: %v", err)
		return
	}
}

func saveToadPhone(ctx telebot.Context, rediska *redis.Client) {
	phoneKey := fmt.Sprintf("toad_notify_phone_%d", ctx.Message().Sender.ID)
	phone := ctx.Message().Contact.PhoneNumber

	// Validation
	phoneRegex := regexp.MustCompile(`\d{10,12}`)
	if !phoneRegex.MatchString(phone) {
		log.Printf("Wrong number: %v", phone)
		err := ctx.Reply(
			"Неверный номер телефона. Поддерживаются только российские номера",
			telebot.RemoveKeyboard,
		)
		if err != nil {
			log.Printf("Phone save wrong reply error: %v", err)
		}
		return
	}

	// Save and welcome
	err := rediska.Set(context.TODO(), phoneKey, phone, 0).Err()
	if err != nil {
		log.Printf("Phone save redis error: %v", err)
		return
	}
	err = ctx.Reply(
		"Номер телефона сохранён.",
		telebot.RemoveKeyboard,
	)
	if err != nil {
		log.Printf("Phone save good reply error: %v", err)
		return
	}
	welcomeToadNotify(ctx, rediska)
}

func welcomeToadNotify(ctx telebot.Context, rediska *redis.Client) {
	isEnabled, err := checkToadNotifyEnabled(rediska, ctx.Message().Sender.ID)
	if err != nil {
		log.Printf("Toad IsEnabled check error: %v", err)
		return
	}
	if isEnabled {
		err = ctx.Reply(
			"Уведомления о работе в кафетерии включены. " +
				"Теперь, когда вам нужно будет приготовить блюдо, бот позвонит вам на мобильный " +
				"с номера +74991135452. " +
				"Поднимать трубку не нужно. ",
		)
		if err != nil {
			log.Printf("Notify enabled reply error: %v", err)
		}
	}
}

func notifyToadCafe(cfg Config, rediska *redis.Client, userId int64) {
	isEnabled, err := checkToadNotifyEnabled(rediska, userId)
	if err != nil {
		log.Printf("Notify: check if enabled error: %v", err)
		return
	}
	phone, err := getToadPhone(rediska, userId)
	if err != nil {
		log.Printf("Notify: get phone error: %v", err)
		return
	}
	if !isEnabled || len(phone) == 0 {
		return
	}
	makeCall(cfg, phone)
}

type NovofonResponse struct {
	Status  string `json:"status"`
	From    string `json:"from"`
	To      string `json:"to"`
	Time    int    `json:"time"`
	Message string `json:"message"`
}

func makeCall(cfg Config, number string) {
	const method = "/v1/request/callback/"
	apiUrl := fmt.Sprintf("https://api.novofon.com%s", method)
	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		log.Printf("MakeCall New Request error: %v", err)
		return
	}

	number = "+" + number
	query := req.URL.Query()
	query.Add("from", cfg.NovofonFrom)
	query.Add("to", number)
	query.Add("predicted", "true")
	req.URL.RawQuery = query.Encode()
	log.Printf("Novafon query: %s", req.URL.RawQuery)

	stringToSign := fmt.Sprintf("%s%s%x", method, query.Encode(), md5.Sum([]byte(query.Encode())))
	hMac := hmac.New(sha1.New, []byte(cfg.NovofonSecret))
	hMac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%x", hMac.Sum(nil))))
	req.Header.Add("Authorization", fmt.Sprintf("%s:%s", cfg.NovofonKey, signature))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Novafon request error: %v", err)
		return
	}
	//goland:noinspection GoUnhandledErrorResult
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Novafon reading body error: %v", err)
		return
	}
	log.Printf("Novafon response: %s", body)
	novafonResponse := NovofonResponse{}
	err = json.Unmarshal(body, &novafonResponse)
	if err != nil {
		log.Printf("Novafon decoding response error: %v", err)
		return
	}
	if novafonResponse.Status == "error" {
		log.Printf("Novafon: error requesting call")
		return
	}
	log.Printf("Novafon: successfully requested call")
}

func checkToadNotifyEnabled(rediska *redis.Client, userId int64) (bool, error) {
	isEnabledKey := fmt.Sprintf("toad_notify_enabled_%d", userId)
	isEnabled, err := rediska.Get(context.TODO(), isEnabledKey).Bool()
	if err != nil && !errors.Is(err, redis.Nil) {
		return false, fmt.Errorf("redis get enabled key error: %w", err)
	}
	return isEnabled, nil
}

func saveStateToadNotify(ctx telebot.Context, rediska *redis.Client, isEnabled bool) error {
	isEnabledKey := fmt.Sprintf("toad_notify_enabled_%d", ctx.Message().Sender.ID)
	err := rediska.Set(context.TODO(), isEnabledKey, isEnabled, 0).Err()
	if err != nil {
		return fmt.Errorf("redis set enabled key error: %w", err)
	}
	log.Printf("Toad notify set to %v for %s", isEnabled, ctx.Message().Sender.FirstName)
	return nil
}

func getToadPhone(rediska *redis.Client, userId int64) (string, error) {
	phoneKey := fmt.Sprintf("toad_notify_phone_%d", userId)
	phone, err := rediska.Get(context.TODO(), phoneKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return "", fmt.Errorf("redis get phone key error: %w", err)
	}
	if len(phone) == 0 || errors.Is(err, redis.Nil) {
		return "", nil
	}
	return phone, nil
}
