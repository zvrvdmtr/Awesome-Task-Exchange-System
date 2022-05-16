package main

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/streadway/amqp"
)

func UserEventsHandler(conn *pgxpool.Pool, messages <-chan amqp.Delivery) {
	for message := range messages {
		var user UserEvent
		err := json.Unmarshal(message.Body, &user)
		if err != nil {
			log.Printf("can`t unmarshal body to struct: %s", err.Error())
		}
		_, err = conn.Exec(context.Background(), "INSERT INTO clients (id, secret, domain) VALUES ($1, $2, $3)", user.ClientID, user.ClientSecret, user.Role)
		if err != nil {
			log.Printf("can`t insert to DB: %s", err.Error())
		}
		_, err = conn.Exec(context.Background(), "INSERT INTO accounts (money, popug_id) VALUES ($1, $2)", 0, user.ClientID)
		if err != nil {
			log.Printf("can`t insert to DB: %s", err.Error())
		}
	}
}

func TaskEventsHandler(conn *pgxpool.Pool, messages <-chan amqp.Delivery, channel *amqp.Channel) {
	for message := range messages {
		var task TaskEvent
		err := json.Unmarshal(message.Body, &task)
		if err != nil {
			log.Printf("can`t unmarshal body to struct: %s", err.Error())
		}
		var money int
		switch task.IsOpen {
		case true:
			money = rand.Intn(-10-(-20)) + (-20)
		case false:
			money = rand.Intn(40-20) + 20
			byteEvent, err := json.Marshal(TaskWithMoneyAndDateEvent{
				Money:    money,
				PublicID: task.PublicID,
				Date:     time.Now(),
			})
			if err != nil {
				log.Printf("can`t marshal struct to body: %s", err.Error())
			}
			channel.Publish(
				"accountingService.TaskStatus",
				"",
				false,
				false,
				amqp.Publishing{
					ContentType: "application/json",
					Body:        byteEvent,
				})
		}

		var accountID int

		row := conn.QueryRow(
			context.Background(),
			"UPDATE accounts SET money=money+$1 WHERE popug_id=$2 RETURNING id", money, task.PopugID)
		if err != nil {
			log.Printf("can`t insert to DB: %s", err.Error())
		}

		err = row.Scan(&accountID)
		if err != nil {
			log.Fatalf("Can't scan row %s", err.Error())
		}

		_, err = conn.Exec(
			context.Background(),
			"INSERT INTO logs (money, public_id, date, account_id) VALUES ($1, $2, $3, $4)", money, task.PublicID, time.Now(), accountID)
		if err != nil {
			log.Printf("can`t insert to DB: %s", err.Error())
		}
	}
}
