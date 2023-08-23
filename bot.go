package main

import (
	"github.com/gin-gonic/gin"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"net/http"
	"sort"
	"strconv"
	"sync"
)

var (
	bot         *tgbotapi.BotAPI
	adminChatID int64
	mu          sync.Mutex
)

func main() {
	var err error
	bot, err = tgbotapi.NewBotAPI("6692426553:AAHFCBXM5aBEuXZ52HXyDC6qwD37ZoFVL2Y")
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true

	log.Printf("Authorized as %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Fatal(err)
	}

	var messages []Message

	go func() {
		for update := range updates {
			if update.Message == nil {
				continue
			}

			log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

			if update.Message.IsCommand() {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
				switch update.Message.Command() {
				case "start":
					msg.Text = "Hello! I am your chatbot. Feel free to ask questions, and administrators will respond."
				case "help":
					msg.Text = "I am a chatbot. Simply ask your question, and an administrator will reply."
				default:
					msg.Text = "I don't understand your command. Please use /help for assistance."
				}
				bot.Send(msg)
			} else {
				mu.Lock()
				messages = append(messages, Message{
					UserName:  update.Message.From.UserName,
					Text:      update.Message.Text,
					IsRead:    false,
					ChatID:    update.Message.Chat.ID,
					MessageID: 0,
				})
				mu.Unlock()
			}
		}
	}()

	r := gin.Default()
	r.LoadHTMLGlob("templates/*")

	r.GET("/", func(c *gin.Context) {
		mu.Lock()
		defer mu.Unlock()

		// Сортируем сообщения по ChatID
		sort.SliceStable(messages, func(i, j int) bool {
			return messages[i].ChatID < messages[j].ChatID
		})

		// Создаем группы сообщений по ChatID
		messageGroups := make(map[int64][]Message)
		for _, msg := range messages {
			messageGroups[msg.ChatID] = append(messageGroups[msg.ChatID], msg)
		}

		c.HTML(http.StatusOK, "messages.html", gin.H{
			"MessageGroups": messageGroups,
		})
	})

	r.POST("/reply", func(c *gin.Context) {
		chatIDStr := c.PostForm("chatID")
		response := c.PostForm("response")

		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid chatID")
			return
		}

		mu.Lock()
		defer mu.Unlock()

		responseSent := false // Флаг для отслеживания отправки ответа

		for i, msg := range messages {
			if msg.ChatID == chatID {
				messages[i].Text = response
				messages[i].IsRead = true

				if !responseSent {
					msg := tgbotapi.NewMessage(chatID, response)
					sentMsg, err := bot.Send(msg) // Отправляем сообщение и сохраняем его
					if err != nil {
						log.Println("Error sending response to Telegram:", err)
					}

					// Связываем исходное сообщение и отправленное сообщение по MessageID
					messages[i].MessageID = sentMsg.MessageID

					responseSent = true // Помечаем, что ответ был отправлен
				}
			}
		}

		c.Redirect(http.StatusSeeOther, "/")
	})

	r.POST("/markread", func(c *gin.Context) {
		messageIDStr := c.PostForm("messageID")

		messageID, err := strconv.Atoi(messageIDStr)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid messageID")
			return
		}

		mu.Lock()
		defer mu.Unlock()

		for i, msg := range messages {
			if msg.MessageID == messageID {
				messages[i].IsRead = true
			}
		}

		c.Redirect(http.StatusSeeOther, "/")
	})
	r.Run(":8088")
}

type Message struct {
	UserName  string
	Text      string
	IsRead    bool
	ChatID    int64
	MessageID int
}
