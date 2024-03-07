package main

import (
	tele "gopkg.in/telebot.v3"
	"log"
)

const welcomeText = `
Здравствуй, путник! И добро пожаловать на Тортугу! 
Мы - маленькое сообщество бывших пикабушников, обосновавшихся на сайте 👉 pipmy.ru 👈

` +
	"🍓 Правила: срачи – во фриспиче, порнуха – в клубничке, маты – везде. " +
	"Спам не любим, если тебя оскорбили, поговори об этом с оппонентом, если же не хочешь вступать в конфликт, то жалуйся админам)" + `

Здесь есть чаты, созвоны и игровые боты
Здесь есть скандалы и интриги
Здесь есть локальные мемы
Здесь уютно, чувствуй себя как дома!
`

const topicsText = `
*Где попиздеть:*
💬 Флудилка – основная тема для общения
⁉️ Фриспич – тоже для общения, но со срачами 

*Остальное:*
🐸 *Жабеночная* – здесь играют в жабобота
#️⃣ *Ботоферма * – чат для игры в остальных ботов (Шма, крокодил)
💡 *Вопросы к администрации* – тех.поддержка сайта pipmy
🍕 *Вкуснотека* – делимся рецептами и фотками еды
🎬 *Кинолекторий (и книги)* – обсуждаем фильмы и книги
🔞 *Клубничка* – делимся эротикой и прочим 😏
🤖 *Гиковская* – обсуждение игр, прогерства и прочего
⭐️ *Новые посты Пипки!* – закрытая тема, бот уведомляет о новых постах на сайте
👨‍🏫 *Политика и новости* – новостные и политические посты, срачи за политику
🦄 *Флора и фауна* – фотки цветов и животных
🎙 *Караокешная* – бот с музыкой
`

const adminsText = `
👨‍⚕️ *Админство:*
[Юнона Соболева-Двачевская](tg://resolve?domain=sobolevadvachevskaya) - создатель
[foxxy07](tg://resolve?domain=foxxy89)
[Egor](tg://resolve?domain=egor3f)
[OmniaMortus](tg://resolve?domain=OmniaMortus)
[Игорь Худяков](tg://resolve?domain=igorkhudiakov)
[AlexMirror](tg://resolve?domain=AlexMirer) 
[Mihail](tg://resolve?domain=ZlobnyTikvo)
[Александр](tg://user?id=1684876579)
[Sergey Gimalaev](tg://resolve?domain=S_Gimalaev)

☎️ *Могут создавать созвоны, но не имеют админки:*
[Billy..X..Bones](tg://resolve?domain=Billy_X_Bones)
[Alena Radost](tg://resolve?domain=Alena_radost)
[Рыба 🐡](tg://resolve?domain=PblbyLa)
`

var welcomeMenu tele.ReplyMarkup
var topicsBtn tele.Btn
var adminsBtn tele.Btn

func init() {
	welcomeMenu = tele.ReplyMarkup{}
	topicsBtn = welcomeMenu.Data("📂 Темы чата", "_welcome_btn_topics")
	adminsBtn = welcomeMenu.Data("🐸 Админы чата", "_welcome_btn_admins")
	welcomeMenu.Inline(
		welcomeMenu.Row(
			topicsBtn,
			adminsBtn,
		),
	)
}

func handleWelcome(bot *tele.Bot) {
	bot.Handle(tele.OnUserJoined, func(ctx tele.Context) error {
		joinedUser := ctx.Message().UserJoined
		log.Printf("User joined: %+v", joinedUser)
		// User: &{
		// 	ID:6393703609 FirstName:Danila LastName:Prohorov IsForum:false Username:dankaravan
		// 	LanguageCode:ru IsBot:false IsPremium:false AddedToMenu:false Usernames:[]
		// 	CustomEmojiStatus: CanJoinGroups:false CanReadMessages:false SupportsInline:false
		// }
		if !joinedUser.IsBot {
			sendWelcome(ctx)
		}
		return nil
	})
	bot.Handle(&topicsBtn, func(ctx tele.Context) error {
		sendTopics(ctx)
		return nil
	})
	bot.Handle(&adminsBtn, func(ctx tele.Context) error {
		sendAdmins(ctx)
		return nil
	})
	bot.Handle("/test_join", func(ctx tele.Context) error {
		sendWelcome(ctx)
		return nil
	})
}

func sendWelcome(ctx tele.Context) {
	err := ctx.Reply(welcomeText, &welcomeMenu)
	if err != nil {
		log.Printf("Welcome reply error: %v", err)
		return
	}
}

func sendTopics(ctx tele.Context) {
	err := ctx.Reply(topicsText)
	if err != nil {
		log.Printf("Topics reply error: %v", err)
		return
	}
}

func sendAdmins(ctx tele.Context) {
	err := ctx.Reply(adminsText, tele.ModeMarkdown)
	if err != nil {
		log.Printf("Admins reply error: %v", err)
		return
	}
}
