package main

import (
	"embed"
	"flag"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"openai/client"
	"os"
	"strconv"
	"strings"
)

var (
	history    = map[string][]client.ChatRequestMessage{}
	MaxHistory int
)

func QA(q string, user string) string {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("run QA time panic: %v", err)
		}
	}()
	msgs := history[user]
	if msgs == nil {
		msgs = []client.ChatRequestMessage{}
	}
	msgs = append(msgs, client.ChatRequestMessage{
		Role:    "user",
		Content: q,
	})
	if len(msgs) > MaxHistory {
		msgs = msgs[len(msgs)-MaxHistory:]
	}
	history[user] = msgs
	chatReq := client.ChatRequest{
		Model:       client.Model,
		Messages:    msgs,
		Temperature: 0.7,
		User:        user,
	}
	err, resp := client.Chat(&chatReq)
	if err != nil {
		fmt.Println(err)
		return "机器人故障"
	}
	a := resp.Choices[0].Message.Content
	msgs = append(msgs, client.ChatRequestMessage{
		Role:    "system",
		Content: a,
	})
	history[user] = msgs
	if true {
		fmt.Println(a)
		printmessage := "输出TOKEN消耗【"
		printmessage += "请求的token数量:" + strconv.Itoa(resp.Usage.PromptTokens)
		printmessage += "，回答的token数量:" + strconv.Itoa(resp.Usage.CompletionTokens)
		printmessage += "，总token数量:" + strconv.Itoa(resp.Usage.TotalTokens)
		printmessage += "】"
		fmt.Println(printmessage)
	}
	return a
}

//go:embed templates/*
var f embed.FS

func runWebService(port int, password string) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("run runWebService time panic: %v", err)
		}
	}()
	gin.SetMode(gin.ReleaseMode)
	routes := gin.Default()
	store := cookie.NewStore([]byte("ckpassworld1234"))
	routes.Use(sessions.Sessions("openaick", store))
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
	_, err := os.Stat("./templates")
	if os.IsNotExist(err) {
		fmt.Printf("使用内置网页模板")
		tmpl := template.New("")
		tmpl = template.Must(tmpl.ParseFS(f, "templates/*.html"))
		routes.SetHTMLTemplate(tmpl)
	} else {
		fmt.Printf("使用templates目录网页模板")
		routes.LoadHTMLGlob("templates/*.html")
	}

	fstatic, _ := fs.Sub(f, "static")
	routes.StaticFS("/static", http.FS(fstatic))

	authorized.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{})
		return
	})

	authorized.GET("/chat/get", func(c *gin.Context) {
		session := sessions.Default(c)
		var ckuser = "default1111"
		ckuserinf := session.Get("user")
		if ckuserinf == nil {
			uuid := uuid.New()
			ckuser = uuid.String()
			session.Set("user", ckuser)
			session.Save()
		} else {
			ckuser = ckuserinf.(string)
		}
		message := c.Query("msg")
		fmt.Println(message)
		a := QA(message, ckuser)
		c.String(http.StatusOK, a)
		return
	})
	routes.Run("0.0.0.0:" + strconv.Itoa(port))
}

func runTgBot(tgbot, tgids string) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("run runTgBot time panic: %v", err)
		}
	}()
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
				a = QA(q, strconv.FormatInt(userid, 10))
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
	historyvar := flag.Int("history", 1, "历史上下文保留,默认为1不保留,如果保留5次对话,就是11，开启历史会增加token使用")
	password := flag.String("password", "", "设置密码开启web验证，用户名为admin")
	socks5 := flag.String("socks5", "", "示例：127.0.0.1:1080")
	tgbot := flag.String("tgbot", "", "tgbot api 没有则不开启")
	tgids := flag.String("tgids", "", "只允许指定的tgid访问,多个id用,分割")
	port := flag.Int("port", 0, "web端口，0则不开启")
	flag.Parse()

	if *historyvar < 1 {
		fmt.Println("history必须>=1")
	}
	MaxHistory = *historyvar

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
	if len(*tgbot) == 0 && *port == 0 {
		fmt.Println("必须启动web或者tgapi")
		return
	}
	select {}
}
