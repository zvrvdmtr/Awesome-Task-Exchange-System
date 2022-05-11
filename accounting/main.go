package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

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
	Description string    `json:"description"`
	IsOpen      bool      `json:"is_open"`
	PopugID     string    `json:"popug_id"`
	PublicID    uuid.UUID `json:"public_id"`
}

type TaskEntity struct {
	ID          int       `db:"id"`
	Description string    `db:"description"`
	IsOpen      bool      `db:"is_open"`
	PublicID    uuid.UUID `db:"public_id"`
	PopugID     string    `db:"popug_id"`
}

func init() {
	conn, _ := pgx.Connect(context.Background(), "postgres://postgres:postgres@localhost:5434/postgres")
	conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS clients (
	  id     	 TEXT  NOT NULL,
	  secret 	 TEXT  NOT NULL,
	  domain 	 TEXT  NOT NULL,
	  CONSTRAINT clients_pkey PRIMARY KEY (id)
	);`)

	conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS tasks (
	  id 			bigserial PRIMARY KEY,
	  description   TEXT 	  NOT NULL,
	  is_open		bool 	  NOT NULL,
	  public_id		UUID 	  NOT NULL,
	  popug_id 		TEXT 	  REFERENCES clients (id)
	);`)
}

func main() {

	//db connection
	conn, err := pgx.Connect(context.Background(), "postgres://postgres:postgres@localhost:5434/postgres")
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
	defer channel.Close()
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
	defer channel.Close()
	taskQueue, err := channel.QueueDeclare("taskStatus", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Can`t declare queue: %s", err.Error())
	}
	err = channel.QueueBind(taskQueue.Name, "", "trackerService.TaskStatus", false, nil)
	if err != nil {
		log.Fatalf("Can`t declare queue: %s", err.Error())
	}
	taskMessages, err := channel.Consume(taskQueue.Name, "", true, false, false, false, nil)

	//start listen rmq
	forever := make(chan bool)
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
			var taskEntity TaskEntity
			row := conn.QueryRow(context.Background(), "SELECT id, description, is_open, popug_id, public_id FROM tasks where public_id = $1", task.PublicID)
			err = row.Scan(&taskEntity.ID, &taskEntity.Description, &taskEntity.IsOpen, &taskEntity.PopugID, &taskEntity.PublicID)
			if err != nil {
				if err == pgx.ErrNoRows {
					_, err = conn.Exec(context.Background(), "INSERT INTO tasks (description, is_open, popug_id, public_id) VALUES ($1, $2, $3, $4)", task.Description, task.IsOpen, task.PopugID, task.PublicID)
					if err != nil {
						log.Printf("can`t insert to DB: %s", err.Error())
					}
				}
			} else {
				fmt.Println(task)
				_, err = conn.Exec(
					context.Background(),
					"UPDATE tasks SET is_open=$1, popug_id=$2 WHERE public_id = $3", task.IsOpen, task.PopugID, task.PublicID)
				if err != nil {
					log.Printf("can`t update DB: %s", err.Error())
				}
			}
		}
	}()

	log.Println("tasks service started on :8081")

	log.Println("Waiting for messages. To exit press CTRL+C")

	<-forever

}
