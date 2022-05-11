package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/go-oauth2/oauth2/v4/models"
	"github.com/streadway/amqp"
	pg "github.com/vgarvardt/go-oauth2-pg/v4"
)

func Registration(clientStore *pg.ClientStore, channel *amqp.Channel) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				log.Printf("can`t read body: %s", err.Error())
			}

			var newUser User
			err = json.Unmarshal(body, &newUser)
			if err != nil {
				log.Printf("can`t unmarshal body to struct: %s", err.Error())
			}

			err = clientStore.Create(&models.Client{
				ID:     newUser.ClientID,
				Secret: newUser.ClientSecret,
				Domain: newUser.Role, // use Domain field as Role field
			})
			if err != nil {
				log.Printf("can`t add new user: %s", err.Error())
			}

			w.Header().Set("Content-Type", "application/json")

			err = channel.Publish(
				"authService.userRegistered",
				"",
				false,
				false,
				amqp.Publishing{
					ContentType: "application/json",
					Body:        body,
				})

			if err != nil {
				log.Printf("failed to publish a message: %s", body)
			}

			json.NewEncoder(w).Encode(map[string]string{
				"CLIENT_ID":     newUser.ClientID,
				"CLIENT_SECRET": newUser.ClientSecret,
				"ROLE":          newUser.Role,
			})
		}
	}
}