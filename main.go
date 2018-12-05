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
	sr := getNextSubreddit(cfg)

	children := getNewestMeme(sr, limit)

	err = setNextSubreddit(cfg, sr)
	if err != nil {
		panic(err)
	}

	for _, u := range children.getURLs() {
		pl := getSlackPayload(u, sr.String())
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
// -------------------------- HELPERS -----------------------------------------
// ----------------------------------------------------------------------------

func sendToSlack(cfg *config.Config, payload map[string]interface{}) {
	pbytes, _ := json.Marshal(payload)

	resp, err := http.Post(cfg.Slack.WebHook, "application/json", bytes.NewBuffer(pbytes))
	if err != nil {
		panic(err)
	}

	msg, _ := ioutil.ReadAll(resp.Body)
	log.Println("status: ", resp.Status, "\nbody: ", string(msg))
}

func getSlackPayload(path, title string) map[string]interface{} {
	atch := []map[string]interface{}{
		map[string]interface{}{"text": title},
	}
	return map[string]interface{}{
		"parse":         "full",
		"response_type": "in_channel",
		"text":          path,
		"title":         title,
		"unfurl_media":  true,
		"unfurl_links":  true,
		"attachments":   atch,
	}
}

func getNewestMeme(sr subreddit, limit int) slackListChildrenSlice {
	client := &http.Client{}
	path := fmt.Sprintf("https://api.reddit.com/r/%v/new?limit=%v", sr.String(), limit)
	req, err := http.NewRequest("POST", path, nil)
	if err != nil {
		panic(err)
	}

	req.Header.Add("User-Agent", "meme-loader-heroku-webbyapp-v1.0.0")
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}

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

func getNextSubreddit(cfg *config.Config) subreddit {
	db := getDB(cfg)
	defer db.Close()

	res, err := db.Query(fmt.Sprintf(`SELECT subreddit FROM tracking WHERE id = '%v'`, cfg.TrackingID))
	if err != nil {
		panic(err)
	}
	defer res.Close()

	subreddit := ""
	if !res.Next() {
		panic("no db record found")
	}

	err = res.Scan(&subreddit)
	if err != nil {
		panic(err)
	}

	switch subreddit {
	case "wholesomememes":
		return WholesomeMemes
	case "me_irl":
		return MeIRL
	default:
		panic("didn't match subreddit")
	}
}

func setNextSubreddit(cfg *config.Config, cur subreddit) error {
	db := getDB(cfg)
	defer db.Close()

	res, err := db.Exec(fmt.Sprintf(`UPDATE tracking SET subreddit = '%v' WHERE id = '%v'`,
		cur.Next().String(),
		cfg.TrackingID))
	if err != nil {
		return err
	}

	id, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if id < 1 {
		log.Println("ERROR - Problem updating.")
	}

	return nil
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
