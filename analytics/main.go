package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/streadway/amqp"
)

func main() {

	err := RunMigrations()
	if err != nil {
		log.Fatalf("Migrations failed: %s", err.Error())
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// db connection
	conn, err := pgxpool.Connect(context.Background(), "postgres://postgres:postgres@localhost:5435/postgres")
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
	moneyMessages, err := channel.Consume(moneyQueue.Name, "", false, false, false, false, nil)

	http.HandleFunc("/daily", DailyResult())
	http.HandleFunc("/task/period", MostExpensiveTaskByPeriod())

	//run service
	go func() {
		if err := http.ListenAndServe(":8082", nil); err != nil {
			log.Fatalf("listen: %s\n", err)
		}
		stop()
	}()
	log.Println("Analytics service started on :8082")

	//start listen rmq
	go UserEventHandler(conn, messages)
	go TaskEventHandler(conn, taskMessages)
	go MoneyEventHandler(conn, moneyMessages)
	log.Println("Waiting for messages. To exit press CTRL+C")

	<-ctx.Done()
}
