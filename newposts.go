package main

import (
	"context"
	"fmt"
	"github.com/mmcdole/gofeed"
	"github.com/redis/go-redis/v9"
	tele "gopkg.in/telebot.v3"
	"log"
)

func notifyNewPosts(cfg Config, bot *tele.Bot, rediska *redis.Client) (err error) {
	const redisSentIdsKey = "notifyNewPosts_sent_ids"
	const redisDryRunKey = "notifyNewPosts_dry_run"

	dryRunKeyExists, err := rediska.Exists(context.TODO(), redisDryRunKey).Result()
	if err != nil {
		return fmt.Errorf("redis dry run error: %w", err)
	}
	dryRun := dryRunKeyExists == 0
	if dryRun {
		log.Println("New posts: Dry run")
		err = rediska.Set(context.TODO(), redisDryRunKey, 1, 0).Err()
		if err != nil {
			return fmt.Errorf("redis dry run set error: %w", err)
		}
	}

	parser := gofeed.NewParser()
	feed, err := parser.ParseURL(cfg.NewPostsFeedURL)
	if err != nil {
		return
	}
	for _, item := range feed.Items {
		isSent, err := rediska.SIsMember(context.TODO(), redisSentIdsKey, item.GUID).Result()
		if err != nil {
			return fmt.Errorf("redis sismember error: %w", err)
		}
		log.Printf("New post: %s, is sent: %v", item.Link, isSent)
		if isSent {
			continue
		}
		if !dryRun {
			message := fmt.Sprintf("Новый пост на pipmy! %s", item.Link)
			for _, threadId := range cfg.NewPostsThreadIds {
				_, err = bot.Send(tele.ChatID(cfg.NewPostsChatId), message, &tele.SendOptions{ThreadID: threadId})
				if err != nil {
					return fmt.Errorf("send error: %w", err)
				}
			}
		}
		if rediska.SAdd(context.TODO(), redisSentIdsKey, item.GUID).Err() != nil {
			return fmt.Errorf("redis sadd error: %w", err)
		}
	}
	return
}
