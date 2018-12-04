package main

import (
	"bytes"
	"log"
	"net/http"
	"os"

	"github.com/mike-webster/meme_loader/config"

	"github.com/gin-gonic/gin"
	_ "github.com/heroku/x/hmetrics/onload"
)

func main() {
	cfg := config.GetConfig("app.json")

	if cfg == nil {
		log.Fatal("Cant parse config")
	}

	port := os.Getenv("PORT")
	if len(port) > 1 {
		cfg.Port = port
	}

	router := gin.New()
	router.Use(gin.Logger())

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, cfg)
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

	router.Run(":" + cfg.Port)
}

func sendToSlack(cfg *config.Config) {
	payload := []byte(`{"text":"Hello, Again!"}`)

	resp, err := http.Post(cfg.Slack.WebHook, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		panic(err)
	}

	log.Println("status: ", resp.Status)
	//curl -X POST -H 'Content-type: application/json' --data '{"text":"Hello, World!"}' https://hooks.slack.com/services/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
}
