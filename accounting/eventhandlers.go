package main

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/streadway/amqp"
)

func UserEventsHandler(conn *pgxpool.Pool, messages <-chan amqp.Delivery) {
	for message := range messages {
		var user UserEvent
		err := json.Unmarshal(message.Body, &user)
		if err != nil {
			log.Printf("can`t unmarshal body to struct: %s", err.Error())
			err = message.Reject(true)
			if err != nil {
				log.Printf("can`t reject message: %s", err.Error())
			}
		} else {
			_, err = conn.Exec(context.Background(), "INSERT INTO clients (secret, domain, popug_id) VALUES ($1, $2, $3)", user.ClientSecret, user.Role, user.ClientID)
			if err != nil {
				log.Printf("can`t insert to DB: %s", err.Error())
				err = message.Reject(true)
				if err != nil {
					log.Printf("can`t reject message: %s", err.Error())
				}
			}
			_, err = conn.Exec(context.Background(), "INSERT INTO accounts (money, popug_id) VALUES ($1, $2)", 0, user.ClientID)
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

func TaskEventsHandler(conn *pgxpool.Pool, messages <-chan amqp.Delivery, channel *amqp.Channel) {
	for message := range messages {
		var task TaskEvent
		err := json.Unmarshal(message.Body, &task)
		if err != nil {
			log.Printf("can`t unmarshal body to struct: %s", err.Error())
			err = message.Reject(true)
			if err != nil {
				log.Printf("can`t reject message: %s", err.Error())
			}
		} else {
			var money int
			switch task.IsOpen {
			case true:
				money = rand.Intn(-10-(-20)) + (-20)
				err = UpdateAccountAndAddLogRecord(conn, money, task.PopugID, task.PublicID)
				if err != nil {
					log.Printf("Error: %s", err.Error())
					err = message.Reject(true)
					if err != nil {
						log.Printf("can`t reject message: %s", err.Error())
					}
				} else {
					message.Ack(false)
				}
			case false:
				money = rand.Intn(40-20) + 20
				err = UpdateAccountAndAddLogRecord(conn, money, task.PopugID, task.PublicID)
				if err != nil {
					log.Printf("Error: %s", err.Error())
					err = message.Reject(true)
					if err != nil {
						log.Printf("can`t reject message: %s", err.Error())
					}
				} else {
					err = PublishTaskEvent(money, task.PublicID, channel)
					if err != nil {
						log.Printf("Error: %s", err.Error())
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
	}
}

func PublishTaskEvent(money int, publicID uuid.UUID, channel *amqp.Channel) error {
	byteEvent, err := json.Marshal(TaskWithMoneyAndDateEvent{
		Money:    money,
		PublicID: publicID,
		Date:     time.Now(),
	})
	if err != nil {
		return err
	}
	err = channel.Publish(
		"accountingService.TaskStatus",
		"",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        byteEvent,
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func UpdateAccountAndAddLogRecord(conn *pgxpool.Pool, money int, popugID string, public uuid.UUID) error {
	var accountID int
	row := conn.QueryRow(
		context.Background(),
		"UPDATE accounts SET money=money+$1 WHERE popug_id=$2 RETURNING id", money, popugID,
	)

	err := row.Scan(&accountID)
	if err != nil {
		log.Printf("Can't scan row %s", err.Error())
		return err
	}

	_, err = conn.Exec(
		context.Background(),
		"INSERT INTO logs (money, public_id, date, account_id) VALUES ($1, $2, $3, $4)", money, public, time.Now(), accountID)
	if err != nil {
		log.Printf("can`t insert to DB: %s", err.Error())
		return err
	}

	return nil
}
