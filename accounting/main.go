package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/streadway/amqp"
)

var (
	ErrSomething  = errors.New("something goes wrong")
	ErrParseToken = errors.New("error parse token")
)

func main() {

	err := RunMigrations()
	if err != nil {
		log.Fatalf("Migrations failed: %s", err.Error())
	}

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

	// publish confirmation
	//channel.Confirm(false)
	//confirmation := make(chan amqp.Confirmation)
	//channel.NotifyPublish(confirmation)

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
	messages, err := channel.Consume(queue.Name, "", false, false, false, false, nil)

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
	taskMessages, err := channel.Consume(taskQueue.Name, "", false, false, false, false, nil)

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
