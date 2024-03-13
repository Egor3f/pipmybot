package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/tg"
	"github.com/redis/go-redis/v9"
	tele "gopkg.in/telebot.v3"
	"log"
	"math/rand"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const antispamRulesKey = "antispam_rules"

type ruleDefinition struct {
	specificPeerId int64
	regex          *regexp.Regexp
}

var specificRuleRegex *regexp.Regexp

func init() {
	specificRuleRegex = regexp.MustCompile(`(\d{6,})\s+(.+)`)
}

func handleAntispam(bot *tele.Bot, cfg Config, rediska *redis.Client) {
	checkPermission := func(ctx tele.Context) bool {
		if !slices.Contains(cfg.AdminIds, ctx.Message().Sender.ID) {
			if err := ctx.Reply("Редактировать параметры антиспама могут только определённые пользователи"); err != nil {
				log.Printf("Reply error: %v", err)
			}
			return false
		}
		return true
	}

	bot.Handle("/antispam", func(ctx tele.Context) error {
		args := strings.TrimSpace(strings.Replace(ctx.Message().Text, "/antispam", "", 1))
		if len(args) == 0 {
			printAntispamRules(ctx, rediska)
		} else {
			if checkPermission(ctx) {
				setAntispamRules(ctx, strings.Split(args, "\n"), rediska)
			}
		}
		return nil
	})
	bot.Handle("/antispam_add", func(ctx tele.Context) error {
		args := strings.TrimSpace(strings.Replace(ctx.Message().Text, "/antispam_add", "", 1))
		if len(args) == 0 {
			err := ctx.Reply("Для добавления правила, используйте команду:\n/antispam_add ID_Бота Регулярное_выражение")
			if err != nil {
				log.Printf("Reply error: %v", err)
			}
			return nil
		}
		if checkPermission(ctx) {
			addAntispamRule(ctx, rediska, args)
		}
		return nil
	})
}

func addAntispamRule(ctx tele.Context, rediska *redis.Client, args string) {
	rules, err := getAntispamRules(rediska)
	if err != nil {
		log.Printf("Adding rules: getting: %v", err)
		return
	}
	rules = append(rules, strings.Split(args, "\n")...)
	setAntispamRules(ctx, rules, rediska)
	printAntispamRules(ctx, rediska)
}

func printAntispamRules(ctx tele.Context, rediska *redis.Client) {
	rules, err := getAntispamRules(rediska)
	if err != nil {
		log.Printf("Print antispam rules error: %v", err)
		return
	}
	rulesStr := strings.Join(rules, "\n")
	err = ctx.Reply(fmt.Sprintf(`
*Текущие правила антиспама:*

`+"```\n%s\n```"+`

Для установки новых правил используйте команду /antispam, далее на каждой новой строке по правилу \(регулярные вырыжения\)\. 
Новые правила заменяют текущие, поэтому есть смысл копировать их из данного сообщения и отредактировать\.
Для установки правила только для определенного пользователя, в начале строки впишите число \(id бота, который можно узнать через @userinfobot\)\. Далее через пробел правило
Для добавления правила, используйте команду: /antispam\_add ID\_Бота Регулярное\_выражение
`, escapeMarkdownV2(rulesStr)), tele.ModeMarkdownV2)
	if err != nil {
		log.Printf("Reply antispam rules error: %v", err)
		return
	}
}

func getAntispamRules(rediska *redis.Client) ([]string, error) {
	rulesStr, err := rediska.Get(context.TODO(), antispamRulesKey).Result()
	if err == nil {
		return strings.Split(rulesStr, "\n"), nil
	} else if errors.Is(err, redis.Nil) {
		return []string{}, nil
	} else {
		return []string{}, fmt.Errorf("get from redis: %w", err)
	}
}

func setAntispamRules(ctx tele.Context, rules []string, rediska *redis.Client) {
	rulesStr := ""
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if len(rule) == 0 {
			continue
		}
		if _, err := parseRule(rule); err != nil {
			err = ctx.Reply(fmt.Sprintf("Ошибка в правиле `%s`\n%s", escapeMarkdownV2(rule), escapeMarkdownV2(err.Error())), tele.ModeMarkdownV2)
			if err != nil {
				log.Printf("Reply error: %v", err)
			}
			return
		}
		rulesStr += rule + "\n"
	}
	rulesStr = strings.TrimSpace(rulesStr)
	if err := rediska.Set(context.TODO(), antispamRulesKey, rulesStr, 0).Err(); err != nil {
		log.Printf("Error settings rules: %v", err)
		if err = ctx.Reply("Ошибка сохранения правил, обратитесь к разработчику"); err != nil {
			log.Printf("Reply error: %v", err)
		}
	}
	if err := ctx.Reply("Правила сохранены"); err != nil {
		log.Printf("Reply error: %v", err)
	}
}

func parseRule(rule string) (ruleDefinition, error) {
	ruleDef := ruleDefinition{}
	var err error
	specificMatches := specificRuleRegex.FindStringSubmatch(rule)
	regStr := rule
	if specificMatches != nil {
		ruleDef.specificPeerId, err = strconv.ParseInt(specificMatches[1], 10, 64)
		if err != nil {
			return ruleDefinition{}, fmt.Errorf("ParseInt error")
		}
		regStr = specificMatches[2]
	}
	ruleDef.regex, err = regexp.Compile(regStr)
	if err != nil {
		return ruleDefinition{}, fmt.Errorf("ошибка в регулярном выражении: %w", err)
	}
	return ruleDef, nil
}

func checkAndRemoveSpam(cfg Config, client *telegram.Client, msg *tg.Message, rediska *redis.Client) {
	clientId, accessHash, err := getChannelData(cfg, client)
	if err != nil {
		log.Printf("Error getting clientId and accessHash: %v", err)
		return
	}
	_ = accessHash

	channel, ok := msg.GetPeerID().(*tg.PeerChannel)
	if !ok {
		log.Printf("Peer %v is not channel", msg.GetPeerID())
		return
	}
	if clientId != channel.GetChannelID() {
		log.Printf("Channel mismatch for %s", msg.Message)
		return
	}
	from, ok := msg.GetFromID()
	if !ok {
		log.Printf("'FromID' is not set")
		return
	}
	user, ok := from.(*tg.PeerUser)
	if !ok {
		log.Printf("From %v is not user", from)
	}

	log.Printf("Checking message for spam: %s", msg.Message)
	ruleStrings, err := getAntispamRules(rediska)
	if err != nil {
		log.Printf("Error loading antispam rules: %v", err)
		return
	}
	for _, ruleStr := range ruleStrings {
		rule, err := parseRule(ruleStr)
		if err != nil {
			log.Printf("Parsing rule %s error: %v", ruleStr, err)
			continue
		}
		if rule.specificPeerId != 0 && rule.specificPeerId != user.GetUserID() {
			continue
		}
		if rule.regex.MatchString(msg.Message) {
			log.Printf("SPAM DETECTED! Message from peer_id: %d and text: %s matches spam rule: %s; to be deleted",
				user.GetUserID(), msg.Message, ruleStr)
			removeMessage(cfg, client, msg)
			break
		}
	}
}

func removeMessage(cfg Config, client *telegram.Client, msg *tg.Message) {
	go func() {
		time.Sleep(time.Duration(rand.Int63n(5000)) * time.Millisecond)
		log.Printf("SPAM DETECTED! Timeout passed; performing delete...")
		sender := message.NewSender(client.API())
		affectedMessages, err := sender.Resolve(cfg.ChannelUsername).Revoke().Messages(context.TODO(), msg.ID)
		/*
			affectedMessages, err := client.API().ChannelsDeleteMessages(context.TO DO(), &tg.ChannelsDeleteMessagesRequest{
				Channel: &tg.InputChannel{
					ChannelID:  channel.GetChannelID(),
					AccessHash: accessHash,
				},
				ID: []int{msg.ID},
			})
		*/
		if err != nil {
			log.Printf("SPAM DETECTED! Error deleting message: %v", err)
		}
		log.Printf("SPAM DETECTED! Deleted message. pts: %d, ptsCount: %d", affectedMessages.Pts, affectedMessages.PtsCount)
	}()
}

var channelIdCache int64
var accessHashCache int64

func getChannelData(cfg Config, client *telegram.Client) (channelId int64, accessHash int64, err error) {
	if channelIdCache > 0 && accessHashCache > 0 {
		return channelIdCache, accessHashCache, nil
	}
	response, err := client.API().ContactsResolveUsername(context.TODO(), cfg.ChannelUsername)
	if err != nil {
		return 0, 0, fmt.Errorf("resolve peer: %w", err)
	}
	if len(response.Chats) == 0 {
		return 0, 0, fmt.Errorf("resolved 0 chats")
	}
	chat, ok := response.Chats[0].(*tg.Channel)
	if !ok {
		return 0, 0, fmt.Errorf("resolved chat is not a channel")
	}
	accessHashTmp, ok := chat.GetAccessHash()
	if !ok {
		return 0, 0, fmt.Errorf("resolved chat has no access hash")
	}
	channelIdCache = chat.GetID()
	accessHashCache = accessHashTmp
	return channelIdCache, accessHashCache, nil
}
