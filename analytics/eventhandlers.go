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

func TaskEventHandler(conn *pgxpool.Pool, messages <-chan amqp.Delivery) {
	for message := range messages {
		var task TaskEvent
		err := json.Unmarshal(message.Body, &task)
		fmt.Println(task)
		if err != nil {
			log.Printf("can`t unmarshal body to struct: %s", err.Error())
			err = message.Reject(true)
			if err != nil {
				log.Printf("can`t reject message: %s", err.Error())
			}
		} else {
			_, err = conn.Exec(
				context.Background(),
				"INSERT INTO closed_tasks (money, public_id, date) VALUES ($1, $2, $3)",
				task.Money,
				task.PublicID,
				task.Date.Format("01-02-2006"),
			)
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

func MoneyEventHandler(conn *pgxpool.Pool, messages <-chan amqp.Delivery) {
	for message := range messages {
		var dailyMoney DailyMoneyEvent
		err := json.Unmarshal(message.Body, &dailyMoney)
		fmt.Println(dailyMoney)
		if err != nil {
			log.Printf("can`t unmarshal body to struct: %s", err.Error())
			err = message.Reject(true)
			if err != nil {
				log.Printf("can`t reject message: %s", err.Error())
			}
		} else {
			_, err = conn.Exec(
				context.Background(),
				"INSERT INTO daily_money (money, popug_id, date) VALUES ($1, $2, $3)",
				dailyMoney.Money,
				dailyMoney.PopugID,
				dailyMoney.Date.Format("01-02-2006"),
			)
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
