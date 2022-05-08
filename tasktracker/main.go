package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-oauth2/oauth2/v4/errors"
	"github.com/jackc/pgx/v4"
	"github.com/streadway/amqp"
)

type UserEvent struct {
	ClientID     string `json:"ClientID"`
	ClientSecret string `json:"ClientSecret"`
	Role         string `json:"Role"`
}

var (
	ErrSomething  = errors.New("Something goes wrong")
	ErrParseToken = errors.New("Error parse token")
)

func init() {
	conn, _ := pgx.Connect(context.Background(), "postgres://postgres:postgres@localhost:5433/postgres")
	conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS users (
	  id     	 TEXT  NOT NULL,
	  secret 	 TEXT  NOT NULL,
	  domain 	 TEXT  NOT NULL,
	  CONSTRAINT clients_pkey PRIMARY KEY (id)
	);`)

	conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS tasks (
	  id 			bigserial primary key,
	  description   TEXT  NOT NULL,
	  is_open		bool   NOT NULL,
	  popug_id 		TEXT REFERENCES clients (id)
	);`)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// db connection
	conn, _ := pgx.Connect(context.Background(), "postgres://postgres:postgres@localhost:5433/postgres")

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

	queue, err := channel.QueueDeclare("task_tracker_service", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Can`t declare queue: %s", err.Error())
	}

	messages, err := channel.Consume(queue.Name, "", true, false, false, false, nil)

	//handlers
	client := http.Client{Timeout: 1 * time.Second}
	http.HandleFunc("/auth", Authorization(&client))
	http.HandleFunc("/tasks", ValidateTokenMiddleware(TasksList(conn), &client))
	http.HandleFunc("/tasks/add", ValidateTokenMiddleware(AddTask(conn), &client))
	http.HandleFunc("/tasks/shuffle", IsAdminMiddleware(ValidateTokenMiddleware(ShuffleTasks(conn), &client), conn))
	http.HandleFunc("/tasks/close", CurrentUserMiddleware(CloseTask(conn), conn))

	//run service
	go func() {
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("listen: %s\n", err)
		}
		stop()
	}()
	log.Println("tasks service started on :8080")

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

	log.Println("Waiting for messages. To exit press CTRL+C")
	<-ctx.Done()
}
