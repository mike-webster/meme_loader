package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/mike-webster/meme_loader/config"

	"github.com/gin-gonic/gin"
	_ "github.com/heroku/x/hmetrics/onload"
)

func main() {
	cfg := config.GetConfig("app.json")

	if cfg == nil {
		log.Fatal("Cant parse config")
	}

	router := gin.New()
	router.Use(gin.Logger())

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, "{'status':'ok'}")
	})

	router.GET("/test", func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// we panicked
				log.Println("we panicked!\n", r)
				c.JSON(http.StatusInternalServerError, "{'message':'unsent'}")
			} else {
				c.JSON(http.StatusOK, "{'message':'sent'}")
			}
		}()

		sendToSlack(cfg)
	})

	router.Run(":" + fmt.Sprint(cfg.Port))
}

func sendToSlack(cfg *config.Config) {
	body := "{'text':'Hello, again!'}"
	bb, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}

	resp, err := http.Post(cfg.Slack.WebHook, "application/json", bytes.NewBuffer(bb))
	if err != nil {
		panic(err)
	}

	fmt.Println("status: ", resp.Status)
	//curl -X POST -H 'Content-type: application/json' --data '{"text":"Hello, World!"}' https://hooks.slack.com/services/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
}
