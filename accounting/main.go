package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/streadway/amqp"
)

type UserEvent struct {
	ClientID     string `json:"ClientID"`
	ClientSecret string `json:"ClientSecret"`
	Role         string `json:"Role"`
}

type TaskEvent struct {
	Description string    `json:"description"`
	IsOpen      bool      `json:"is_open"`
	PopugID     string    `json:"popug_id"`
	PublicID    uuid.UUID `json:"public_id"`
}

type TaskWithMoneyAndDateEvent struct {
	PopugID  string
	PublicID uuid.UUID
	Money    int
	Date     time.Time
}

type DailyMoneyEvent struct {
	PopugID string
	Money   int
	Date    time.Time
}

type User struct {
	ClientID     string `json:"ClientID"`
	ClientSecret string `json:"ClientSecret"`
}

type LogRecordEntity struct {
	ID        int
	Money     int
	PublicID  uuid.UUID
	Date      time.Time
	AccountID int
}

type AccountEntity struct {
	ID      int
	Money   int
	PopugID int
}

type AccountWithLogs struct {
	Account AccountEntity
	Logs    []LogRecordEntity
}

var (
	ErrSomething  = errors.New("something goes wrong")
	ErrParseToken = errors.New("error parse token")
)

func init() {
	conn, _ := pgx.Connect(context.Background(), "postgres://postgres:postgres@localhost:5434/postgres")
	conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS clients (
	  id     	 TEXT  NOT NULL,
	  secret 	 TEXT  NOT NULL,
	  domain 	 TEXT  NOT NULL,
	  CONSTRAINT clients_pkey PRIMARY KEY (id)
	);`)

	_, err := conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS accounts (
	  id 			bigserial PRIMARY KEY,
	  money		    INTEGER   NOT NULL,
	  popug_id 		TEXT 	  REFERENCES clients (id) UNIQUE
	);`)
	fmt.Println(err)

	_, err = conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS logs (
	  id 			bigserial PRIMARY KEY,
	  money		    INTEGER   NOT NULL,
	  public_id		UUID 	  NOT NULL,
	  date			TIMESTAMP NOT NULL,
	  account_id 	INTEGER   REFERENCES accounts (id)
	);`)
	fmt.Println(err)
}

func main() {

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	//db connection
	connPool, err := pgxpool.Connect(context.Background(), "postgres://postgres:postgres@localhost:5434/postgres")
	//conn, err := pgx.Connect(context.Background(), "postgres://postgres:postgres@localhost:5434/postgres")
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
	queue, err := channel.QueueDeclare("userRegistered", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Can`t declare queue: %s", err.Error())
	}
	err = channel.QueueBind(queue.Name, "", "authService.userRegistered", false, nil)
	if err != nil {
		log.Fatalf("Can`t declare queue: %s", err.Error())
	}
	messages, err := channel.Consume(queue.Name, "", true, false, false, false, nil)

	// task events
	err = channel.ExchangeDeclare("trackerService.TaskStatus", "fanout", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Can`t create exchange: %s", err.Error())
	}
	taskQueue, err := channel.QueueDeclare("taskStatus", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Can`t declare queue: %s", err.Error())
	}
	err = channel.QueueBind(taskQueue.Name, "", "trackerService.TaskStatus", false, nil)
	if err != nil {
		log.Fatalf("Can`t declare queue: %s", err.Error())
	}
	taskMessages, err := channel.Consume(taskQueue.Name, "", true, false, false, false, nil)

	// money events
	err = channel.ExchangeDeclare("accountingService.TaskStatus", "fanout", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Can`t create exchange: %s", err.Error())
	}

	client := http.Client{Timeout: 1 * time.Second}
	http.HandleFunc("/auth", Authorization(&client))
	http.HandleFunc("/account", ValidateTokenMiddleware(PopugAccount(connPool), &client))
	http.HandleFunc("/account/admin", AllAccounts(connPool))
	http.HandleFunc("/daily", DailyResult(connPool, channel))
	//http.HandleFunc("/account/admin", IsAdminMiddleware(AllAccounts(conn), conn))

	//run service
	go func() {
		if err := http.ListenAndServe(":8081", nil); err != nil {
			log.Fatalf("listen: %s\n", err)
		}
		stop()
	}()
	log.Println("Account service started on :8081")

	go UserEventsHandler(connPool, messages)
	go TaskEventsHandler(connPool, taskMessages, channel)
	log.Println("Waiting for messages. To exit press CTRL+C")

	<-ctx.Done()

}
