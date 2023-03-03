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
	"gopkg.in/yaml.v3"
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
	history            = map[string][]client.ChatRequestMessage{}
	historyAccessToken = map[string]client.ChatText{}
	MaxHistory         int
	useAPI             = true
)

func QA(q string, user string) string {
	if useAPI {
		return QAByAPI(q, user)
	} else {
		return QAByAccessToken(q, user)
	}
}

func QAByAPI(q string, user string) string {
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

func QAByAccessToken(q string, user string) string {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("run QAByAccessToken time panic: %v", err)
		}
	}()
	last := historyAccessToken[user]
	now, err := client.GetChatText(q, last.ConversationID, last.MessageID)
	if err != nil {
		fmt.Println(err)
		return "机器人故障"
	}
	historyAccessToken[user] = *now
	return now.Content
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
		tmpl := template.New("")
		tmpl = template.Must(tmpl.ParseFS(f, "templates/*.html"))
		routes.SetHTMLTemplate(tmpl)
	} else {
		fmt.Println("加载本地templates/index.html模板")
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

type Config struct {
	APIKEY          string `yaml:"APIKEY"`
	Token           string `yaml:"token"`
	Socks5          string `yaml:"socks5"`
	Model           string `yaml:"model"`
	History         int    `yaml:"history"`
	Port            int    `yaml:"port"`
	Password        string `yaml:"password"`
	Tgbot           string `yaml:"tgbot"`
	Tgids           string `yaml:"tgids"`
	ReverseProxyURL string `yaml:"reverseproxyurl"`
}

func main() {
	apikey := flag.String("APIKEY", "", "APIKEY/token,必须使用一个")
	token := flag.String("token", "", "APIKEY/token,必须使用一个")
	socks5 := flag.String("socks5", "", "[通用参数]示例：127.0.0.1:1080")
	model := flag.String("model", "gpt-3.5-turbo", "[API参数]指定模型")
	historyvar := flag.Int("history", 1, "[API参数]历史上下文保留,默认为1不保留,如果保留5次对话,就是11，开启历史会增加token使用")
	port := flag.Int("port", 0, "web端口，0则不开启")
	password := flag.String("password", "", "[web参数]设置密码开启web验证，用户名为admin")
	tgbot := flag.String("tgbot", "", "[tgbot参数]tgbot api 没有则不开启")
	tgids := flag.String("tgids", "", "[tgbot参数]只允许指定的tgid访问,多个id用,分割")
	reverseproxyurl := flag.String("reverseproxyurl", "https://chat.duti.tech/api/conversation", "120 req/min by IP")
	flag.Parse()

	data, err := os.ReadFile("config.yaml")
	if err != nil {
		fmt.Println("当前目录不存在配置config.yaml,使用命令行参数")
	} else {
		fmt.Println("使用config.yaml")
	}
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		fmt.Println("解析本地config.json失败", err)
		return
	} else {
		if len(config.APIKEY) > 0 {
			apikey = &config.APIKEY
		}
		if len(config.Token) > 0 {
			token = &config.Token
		}
		if len(config.Socks5) > 0 {
			socks5 = &config.Socks5
		}
		if len(config.Model) > 0 {
			model = &config.Model
		}
		if config.History > 1 {
			historyvar = &config.History
		}
		if config.Port > 0 {
			port = &config.Port
		}
		if len(config.Password) > 0 {
			password = &config.Password
		}
		if len(config.Tgbot) > 0 {
			tgbot = &config.Tgbot
		}
		if len(config.Tgids) > 0 {
			tgids = &config.Tgids
		}
		if len(config.ReverseProxyURL) > 0 {
			reverseproxyurl = &config.ReverseProxyURL
		}
	}

	if len(*apikey) == 0 && len(*token) == 0 {
		fmt.Println("必须指定APIKEY/token二选一")
		return
	} else {
		if len(*apikey) > 0 {
			client.API_KEY = *apikey
			client.Model = *model
			if *historyvar < 1 {
				fmt.Println("history必须>=1")
				return
			}
			MaxHistory = *historyvar
			fmt.Println("使用APIKEY")
		}
		if len(*token) > 0 {
			client.AccessToken = *token
			client.ReverseProxyURL = *reverseproxyurl
			useAPI = false
			fmt.Println("使用Token")
		}
	}
	client.InitCuzClient(*socks5)
	if len(*tgbot) > 0 {
		go runTgBot(*tgbot, *tgids)
	}
	if *port > 0 {
		go runWebService(*port, *password)
	}
	if len(*tgbot) == 0 && *port == 0 {
		fmt.Println("必须启动web/tgapi二选一")
		return
	}
	select {}
}
