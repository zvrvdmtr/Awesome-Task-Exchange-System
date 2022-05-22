package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/streadway/amqp"
)

func UserEventHandler(conn *pgxpool.Pool, messages <-chan amqp.Delivery) {
	for message := range messages {
		var user UserEvent
		err := json.Unmarshal(message.Body, &user)
		fmt.Println(user)
		if err != nil {
			log.Printf("can`t unmarshal body to struct: %s", err.Error())
			err = message.Reject(true)
			if err != nil {
				log.Printf("can`t reject message: %s", err.Error())
			}
		} else {
			_, err = conn.Exec(context.Background(), "INSERT INTO clients (id, secret, domain) VALUES ($1, $2, $3)", user.ClientID, user.ClientSecret, user.Role)
			if err != nil {
				log.Printf("can`t insert to DB: %s", err.Error())
				err = message.Reject(true)
				if err != nil {
					log.Printf("can`t reject message: %s", err.Error())
				}
			} else {
				message.Ack(false)
			}
		}

	}
}
