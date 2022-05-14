package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/streadway/amqp"
)

type UserEvent struct {
	ClientID     string `json:"ClientID"`
	ClientSecret string `json:"ClientSecret"`
	Role         string `json:"Role"`
}

type TaskEvent struct {
	Money    int
	Date     time.Time
	PublicID uuid.UUID
}

type DailyMoneyEvent struct {
	Money   int
	Date    time.Time
	PopugID string
}

func init() {
	conn, _ := pgx.Connect(context.Background(), "postgres://postgres:postgres@localhost:5435/postgres")
	conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS clients (
	  id     	 TEXT  NOT NULL,
	  secret 	 TEXT  NOT NULL,
	  domain 	 TEXT  NOT NULL,
	  CONSTRAINT clients_pkey PRIMARY KEY (id)
	);`)

	_, err := conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS closed_tasks (
	  id 			bigserial PRIMARY KEY,
	  money		    INTEGER   NOT NULL,
	  public_id		UUID 	  NOT NULL,
	  date			DATE	  NOT NULL
	);`)
	if err != nil {
		fmt.Println(err)
	}

	_, err = conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS daily_money (
	  id 			bigserial PRIMARY KEY,
	  money		    INTEGER   NOT NULL,
	  popug_id		TEXT 	  NOT NULL,
	  date			DATE	  NOT NULL
	);`)
	if err != nil {
		fmt.Println(err)
	}
}

func main() {

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// db connection
	conn, err := pgx.Connect(context.Background(), "postgres://postgres:postgres@localhost:5435/postgres")
	if err != nil {
		log.Fatalf("Can`t connect to DB: %s", err.Error())
	}

	// rmq connection
	rabbitConn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Fatalf("Can`t connect to RabbitMQ: %s", err.Error())
	}
	defer rabbitConn.Close()
	channel, err := rabbitConn.Channel()
	if err != nil {
		log.Fatalf("Can`t create channel: %s", err.Error())
	}
	defer channel.Close()

	// user events
	err = channel.ExchangeDeclare("authService.userRegistered", "fanout", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Can`t create exchange: %s", err.Error())
	}
	queue, err := channel.QueueDeclare("", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Can`t declare queue: %s", err.Error())
	}
	err = channel.QueueBind(queue.Name, "", "authService.userRegistered", false, nil)
	if err != nil {
		log.Fatalf("Can`t declare queue: %s", err.Error())
	}
	messages, err := channel.Consume(queue.Name, "", true, false, false, false, nil)

	// task events
	err = channel.ExchangeDeclare("accountingService.TaskStatus", "fanout", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Can`t create exchange: %s", err.Error())
	}
	taskQueue, err := channel.QueueDeclare("", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Can`t declare queue: %s", err.Error())
	}
	err = channel.QueueBind(taskQueue.Name, "", "accountingService.TaskStatus", false, nil)
	if err != nil {
		log.Fatalf("Can`t declare queue: %s", err.Error())
	}
	taskMessages, err := channel.Consume(taskQueue.Name, "", true, false, false, false, nil)

	// money events
	err = channel.ExchangeDeclare("accountingService.DailyMoney", "fanout", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Can`t create exchange: %s", err.Error())
	}
	moneyQueue, err := channel.QueueDeclare("", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Can`t declare queue: %s", err.Error())
	}
	err = channel.QueueBind(moneyQueue.Name, "", "accountingService.DailyMoney", false, nil)
	if err != nil {
		log.Fatalf("Can`t declare queue: %s", err.Error())
	}
	moneyMessages, err := channel.Consume(moneyQueue.Name, "", true, false, false, false, nil)

	http.HandleFunc("/daily", DailyResult())
	http.HandleFunc("/daily", mostExpensiveTaskByPeriod())

	//run service
	go func() {
		if err := http.ListenAndServe(":8081", nil); err != nil {
			log.Fatalf("listen: %s\n", err)
		}
		stop()
	}()
	log.Println("Analytics service started on :8082")

	//start listen rmq
	go func() {
		for message := range messages {
			var user UserEvent
			err = json.Unmarshal(message.Body, &user)
			if err != nil {
				log.Printf("can`t unmarshal body to struct: %s", err.Error())
			}
			_, err = conn.Exec(context.Background(), "INSERT INTO clients (id, secret, domain) VALUES ($1, $2, $3)", user.ClientID, user.ClientSecret, user.Role)
			if err != nil {
				log.Printf("can`t insert to DB: %s", err.Error())
			}
		}
	}()

	go func() {
		for taskMessage := range taskMessages {
			var task TaskEvent
			err = json.Unmarshal(taskMessage.Body, &task)
			if err != nil {
				log.Printf("can`t unmarshal body to struct: %s", err.Error())
			}
			_, err = conn.Exec(
				context.Background(),
				"INSERT INTO closed_tasks (money, public_id, date) VALUES ($1, $2, $3)",
				task.Money,
				task.PublicID,
				task.Date.Format("01-02-2006"),
			)
			if err != nil {
				log.Printf("can`t insert to DB: %s", err.Error())
			}
		}
	}()

	go func() {
		for moneyMessage := range moneyMessages {
			var dailyMoney DailyMoneyEvent
			err = json.Unmarshal(moneyMessage.Body, &dailyMoney)
			if err != nil {
				log.Printf("can`t unmarshal body to struct: %s", err.Error())
			}
			_, err = conn.Exec(
				context.Background(),
				"INSERT INTO daily_money (money, popug_id, date) VALUES ($1, $2, $3)",
				dailyMoney.Money,
				dailyMoney.PopugID,
				dailyMoney.Date.Format("01-02-2006"),
			)
			if err != nil {
				log.Printf("can`t insert to DB: %s", err.Error())
			}
		}
	}()
	log.Println("Waiting for messages. To exit press CTRL+C")

	<-ctx.Done()
}
