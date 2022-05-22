package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/streadway/amqp"
)

// TODO add context to handlers
// TODO move rmq connection to separate file
// TODO pass data from middleware to handler
// TODO add docker compose file

func main() {

	err := RunMigrations()
	if err != nil {
		log.Fatalf("Migrations failed: %s", err.Error())
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// db connection
	conn, _ := pgxpool.Connect(context.Background(), "postgres://postgres:postgres@localhost:5433/postgres")

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

	// publish confirmation
	channel.Confirm(false)
	confirmation := make(chan amqp.Confirmation)
	channel.NotifyPublish(confirmation)

	err = channel.ExchangeDeclare("authService.userRegistered", "fanout", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Can`t create exchange: %s", err.Error())
	}
	defer channel.Close()

	err = channel.ExchangeDeclare("trackerService.TaskStatus", "fanout", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Can`t create exchange: %s", err.Error())
	}
	defer channel.Close()

	queue, err := channel.QueueDeclare("", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Can`t declare queue: %s", err.Error())
	}

	err = channel.QueueBind(queue.Name, "", "authService.userRegistered", false, nil)
	if err != nil {
		log.Fatalf("Can`t declare queue: %s", err.Error())
	}

	messages, err := channel.Consume(queue.Name, "", false, false, false, false, nil)

	//handlers
	client := http.Client{Timeout: 1 * time.Second}
	http.HandleFunc("/auth", Authorization(&client))
	http.HandleFunc("/tasks", ValidateTokenMiddleware(TasksList(conn), &client))
	http.HandleFunc("/tasks/add", ValidateTokenMiddleware(AddTask(conn, channel, &client, confirmation), &client))
	http.HandleFunc("/tasks/shuffle", IsAdminMiddleware(ValidateTokenMiddleware(ShuffleTasks(conn, channel, confirmation), &client), conn))
	http.HandleFunc("/tasks/close", CurrentUserMiddleware(CloseTask(conn, channel, confirmation), conn))

	//run service
	go func() {
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("listen: %s\n", err)
		}
		stop()
	}()
	log.Println("tasks service started on :8080")

	//start listen rmq
	go UserEventHandler(conn, messages)

	log.Println("Waiting for messages. To exit press CTRL+C")
	<-ctx.Done()
}
