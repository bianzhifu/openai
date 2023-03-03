package main

import (
	"embed"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"openai/client"
	"strconv"
	"strings"
)

func QA(q string) string {
	if err := recover(); err != nil {
		fmt.Println(err)
		return "机器人故障"
	}
	chatReq := client.ChatRequest{
		Model: client.Model,
		Messages: []client.ChatRequestMessage{{Role: "user",
			Content: q},
		},
	}
	err, resp := client.Chat(&chatReq)
	if err != nil {
		fmt.Println(err)
		return "机器人故障"
	}
	a := resp.Choices[0].Message.Content
	if false {
		a = a + "PromptTokens:" + strconv.Itoa(resp.Usage.PromptTokens)
		a = a + "CompletionTokens:" + strconv.Itoa(resp.Usage.CompletionTokens)
		a = a + "TotalTokens:" + strconv.Itoa(resp.Usage.TotalTokens)
	}
	return a
}

//go:embed templates/*
var f embed.FS

func runWebService(port int, password string) {
	gin.SetMode(gin.ReleaseMode)
	routes := gin.Default()
	routes.Static("/movie", "./movie/")

	var authorized *gin.RouterGroup
	if len(password) > 0 {
		ginauth := gin.BasicAuth(
			gin.Accounts{"admin": password},
		)
		authorized = routes.Group("/", ginauth)
	} else {
		authorized = routes.Group("/")
	}
	tmpl := template.New("")
	tmpl = template.Must(tmpl.ParseFS(f, "templates/*.html"))
	routes.SetHTMLTemplate(tmpl)

	fstatic, _ := fs.Sub(f, "static")
	routes.StaticFS("/static", http.FS(fstatic))

	authorized.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{})
		return
	})

	authorized.GET("/chat/get", func(c *gin.Context) {
		message := c.Query("msg")
		a := QA(message)
		c.String(http.StatusOK, a)
		return
	})
	routes.Run("0.0.0.0:" + strconv.Itoa(port))
}

func runTgBot(tgbot, tgids string) {
	bot, err := tgbotapi.NewBotAPIWithClient(tgbot, tgbotapi.APIEndpoint, client.CuzClient)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil { // If we got a message
			log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
			userid := update.Message.From.ID
			q := update.Message.Text
			a := "没有权限访问chatgpt"
			if len(tgids) == 0 || strings.Contains(","+tgids+",", ","+strconv.FormatInt(userid, 10)+",") {
				a = QA(q)
			}
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, a)
			msg.ReplyToMessageID = update.Message.MessageID
			bot.Send(msg)
		}
	}
}

func main() {

	apikey := flag.String("APIKEY", "", "APIKEY,必须指定")
	model := flag.String("model", "gpt-3.5-turbo", "指定模型")
	password := flag.String("password", "", "设置密码开启web验证，用户名为admin")
	socks5 := flag.String("socks5", "", "示例：127.0.0.1:1080")
	tgbot := flag.String("tgbot", "", "tgbot api 没有则不开启")
	tgids := flag.String("tgids", "", "只允许指定的tgid访问,多个id用,分割")
	port := flag.Int("port", 0, "web端口，0则不开启")
	flag.Parse()

	if len(*apikey) == 0 {
		fmt.Println("必须指定APIKEY")
		return
	} else {
		client.InitApi(*apikey, *model)
	}

	client.InitCuzClient(*socks5)
	if len(*tgbot) > 0 {
		go runTgBot(*tgbot, *tgids)
	}
	if *port > 0 {
		go runWebService(*port, *password)
	}
	select {}
}
