package main

import (
	"context"
	"fmt"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/telegram/updates/hook"
	"github.com/gotd/td/tg"
	"log"
)

type messageHandler func(update *tg.UpdateNewChannelMessage)

func monitorBotMessages(cfg Config, onNewMessage messageHandler) {
	dispatcher := tg.NewUpdateDispatcher()
	upd := updates.New(updates.Config{
		Handler: dispatcher,
	})

	client := telegram.NewClient(
		cfg.TelegramClient.AppId,
		cfg.TelegramClient.AppHash,
		telegram.Options{
			SessionStorage: &telegram.FileSessionStorage{Path: "./session.json"},
			Middlewares: []telegram.Middleware{
				hook.UpdateHook(upd.Handle),
			},
			UpdateHandler: upd,
		},
	)

	dispatcher.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewChannelMessage) error {
		onNewMessage(update)
		return nil
	})

	if err := client.Run(context.TODO(), func(ctx context.Context) error {
		//authorizeClient(client)
		api := client.API()
		self, err := client.Self(ctx)
		if err != nil {
			return fmt.Errorf("getting tg self info: %w", err)
		}
		return upd.Run(ctx, api, self.ID, updates.AuthOptions{
			OnStart: func(ctx context.Context) {
				log.Printf("Telegram client started listening for updates...")
			},
		})
	}); err != nil {
		log.Printf("Telegram client error: %v", err)
	}
}

/*
func authorizeClient(client *telegram.Client) {
	codePrompt := func(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
		fmt.Print("Enter code: ")
		code, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(code), nil
	}

	fmt.Print("Enter phone: ")
	phone, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		log.Printf("Error reading stdio: %v", err)
		return
	}
	fmt.Printf("Phone: %s\n", phone)
	err = auth.NewFlow(
		auth.CodeOnly(phone, auth.CodeAuthenticatorFunc(codePrompt)),
		auth.SendCodeOptions{
			AllowFlashCall: true,
		},
	).Run(context.T ODO(), client.Auth())
	if err != nil {
		log.Printf("Error authenticating telegram: %v", err)
	}
}
*/
