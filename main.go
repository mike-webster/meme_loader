package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"github.com/mike-webster/meme_loader/config"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/jmoiron/sqlx"
)

func main() {
	cfg := config.GetConfig("app.json")

	if cfg == nil {
		log.Fatal("Cant parse config")
	}

	router := gin.New()
	router.Use(gin.Logger())

	router.GET("/", healthcheckHandler)

	router.GET("/send", sendHandler)

	router.Run(":" + cfg.Port)
}

func sendToSlack(cfg *config.Config, payload map[string]interface{}) {
	pbytes, _ := json.Marshal(payload)

	resp, err := http.Post(cfg.Slack.WebHook, "application/json", bytes.NewBuffer(pbytes))
	if err != nil {
		panic(err)
	}

	log.Println("status: ", resp.Status)
	//curl -X POST -H 'Content-type: application/json' --data '{"text":"Hello, World!"}' https://hooks.slack.com/services/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
}

func getSlackPayload(path string) map[string]interface{} {
	return map[string]interface{}{
		"parse":         "full",
		"response_type": "in_channel",
		"text":          path,
		"unfurl_media":  true,
		"unfurl_links":  true,
	}
}

func getNewestWholesomeMemes(limit int) slackListChildrenSlice {
	client := &http.Client{}
	path := fmt.Sprint("https://api.reddit.com/r/wholesomememes/new?limit=", limit)
	req, err := http.NewRequest("POST", path, nil)
	if err != nil {
		panic(err)
	}

	req.Header.Add("User-Agent", "meme-loader-heroku-webbyapp-v1.0.0")

	resp, err := client.Do(req)

	if resp.StatusCode != http.StatusOK {
		panic(fmt.Sprint("bad response: ", resp.Status))
	}

	var ret slackResp
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(respBytes, &ret)
	if err != nil {
		panic(err)
	}

	return ret.Data.Children
}

// ----------------------------------------------------------------------------
// -------------------------- HANDLERS ----------------------------------------
// ----------------------------------------------------------------------------

func sendHandler(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			// we panicked
			log.Println("we panicked!\n", r)
			c.JSON(http.StatusInternalServerError, "{'message':'unsent'}")
		} else {
			c.JSON(http.StatusOK, "{'message':'sent'}")
		}
	}()

	limit, err := strconv.Atoi(c.Query("number"))
	if err != nil {
		limit = 1
	}

	cfg := config.GetConfig("app.json")
	children := getNewestWholesomeMemes(limit)

	for _, u := range children.getURLs() {
		pl := getSlackPayload(u)
		log.Println(pl)
		sendToSlack(cfg, pl)
	}
}

func healthcheckHandler(c *gin.Context) {
	cfg := config.GetConfig("app.json")
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, "{'err':'couldnt parse config'}")
		return
	}

	db := getDB(cfg)
	if db == nil {
		c.JSON(http.StatusInternalServerError, `{"err":"couldn't open database"}`)
		return
	}

	c.JSON(http.StatusOK, `{"msg":"everything ok!"}`)
}

// ----------------------------------------------------------------------------
// -------------------------- DATA --------------------------------------------
// ----------------------------------------------------------------------------
func getDB(cfg *config.Config) *sqlx.DB {
	db, err := sqlx.Connect("mysql", fmt.Sprintf("%v:%v@tcp(%v:3306)/meme_loader",
		cfg.Database.DbUser,
		cfg.Database.DbPass,
		cfg.Database.DbHost))

	if err != nil {
		log.Fatal(err)
	}

	return db
}

// ----------------------------------------------------------------------------
// -------------------------- STRUCTS -----------------------------------------
// ----------------------------------------------------------------------------

type subreddit int

const (
	WholesomeMemes subreddit = iota
	MeIRL
)

func (s subreddit) String() string {
	switch s {
	case WholesomeMemes:
		return "wholesomememes"
	case MeIRL:
		return "me_irl"
	default:
		panic("value not setup")
	}
}

func (s subreddit) Next() subreddit {
	switch s {
	case WholesomeMemes:
		return MeIRL
	case MeIRL:
		return WholesomeMemes
	default:
		panic("value not setup")
	}
}

type slackResp struct {
	Data slackRespData `json:"data"`
}

type slackRespData struct {
	After    string `json:"after"`
	Before   string `json:"before"`
	Children slackListChildrenSlice
}

type slackListChildren struct {
	Kind  string         `json:"kind"`
	Child slackListChild `json:"data"`
}

type slackListChild struct {
	SubReddit string `json:"subreddit"`
	Thumbnail string `json:"thumbnail"`
	URL       string `json:"URL"`
}

type slackListChildrenSlice []slackListChildren

func (slc *slackListChildrenSlice) getURLs() []string {
	ret := []string{}
	for _, i := range *slc {
		ret = append(ret, i.Child.URL)
	}

	return ret
}
