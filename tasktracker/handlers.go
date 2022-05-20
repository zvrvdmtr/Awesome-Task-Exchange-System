package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/streadway/amqp"
)

type User struct {
	ClientID     string `json:"ClientID"`
	ClientSecret string `json:"ClientSecret"`
}

type Task struct {
	Description string `json:"description"`
	JiraID      string `json:"jira_id"`
	IsOpen      bool   `json:"is_open"`
	PopugID     string `json:"popug_id"`
}

type TaskEvent struct {
	JiraID      string    `json:"jira_id"`
	Description string    `json:"description"`
	IsOpen      bool      `json:"is_open"`
	PopugID     string    `json:"popug_id"`
	PublicID    uuid.UUID `json:"public_id"`
}

func Authorization(client *http.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method == http.MethodPost {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				log.Printf("can`t read body: %s", err.Error())
			}

			var UserData User
			err = json.Unmarshal(body, &UserData)
			if err != nil {
				log.Printf("can`t unmarshal body to struct: %s", err.Error())
			}

			req, err := http.NewRequest("GET", "http://localhost:9096/token", nil)
			if err != nil {
				log.Printf("can`t build new request: %s", err.Error())
			}

			q := req.URL.Query()
			q.Add("grant_type", "client_credentials")
			q.Add("client_id", UserData.ClientID)
			q.Add("client_secret", UserData.ClientSecret)
			q.Add("scope", "read")
			req.URL.RawQuery = q.Encode()

			resp, err := client.Do(req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
			if resp.StatusCode != http.StatusOK {
				http.Error(w, ErrSomething.Error(), resp.StatusCode)
			}
			response, err := ioutil.ReadAll(resp.Body)
			w.Header().Set("Content-Type", "application/json")

			json.NewEncoder(w).Encode(string(response))
		}
	}
}

func TasksList(connection *pgx.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		claims, err := parseToken(token)
		if err != nil {
			http.Error(w, ErrParseToken.Error(), http.StatusBadRequest)
			return
		}
		rows, err := connection.Query(
			context.Background(),
			"SELECT id, jira_id, description, is_open, popug_id FROM tasks where popug_id = $1",
			claims.StandardClaims.Audience,
		)
		if err != nil {
			http.Error(w, ErrSomething.Error(), http.StatusInternalServerError)
		}
		tasks := make([]TaskEntity, 0)
		for rows.Next() {
			var task TaskEntity
			err = rows.Scan(&task.ID, &task.JiraID, &task.Description, &task.IsOpen, &task.PopugID)
			if err != nil {
				log.Printf("error while parse: %s", err.Error())
			}
			tasks = append(tasks, task)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tasks)
	}
}

func AddTask(connection *pgx.Conn, channel *amqp.Channel, client *http.Client, confirmation chan amqp.Confirmation) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				http.Error(w, ErrSomething.Error(), http.StatusInternalServerError)
			}

			var task Task
			err = json.Unmarshal(body, &task)
			if err != nil {
				http.Error(w, ErrSomething.Error(), http.StatusInternalServerError)
			}

			taskUUID := uuid.New()
			taskEvent := fromTaskToEvent(task, taskUUID)

			_, err = connection.Exec(
				context.Background(),
				"INSERT INTO tasks (description, jira_id, is_open, popug_id, public_id) VALUES ($1, $2, $3, $4, $5)",
				task.Description,
				task.JiraID,
				task.IsOpen,
				task.PopugID,
				taskUUID,
			)
			if err != nil {
				log.Printf("can`t insert to DB: %s", err.Error())
			}

			byteEvent, err := json.Marshal(taskEvent)
			if err != nil {
				http.Error(w, ErrSomething.Error(), http.StatusInternalServerError)
			}
			log.Print(string(byteEvent))

			request, err := http.NewRequest("POST", "http://localhost:8083/validate_task", bytes.NewReader(byteEvent))
			if err != nil {
				log.Printf("can`t build new request: %s", err.Error())
			}
			resp, err := client.Do(request)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
			if resp.StatusCode != http.StatusOK {
				http.Error(w, ErrInvalidSchema.Error(), http.StatusBadRequest)
			}

			err = channel.Publish(
				"trackerService.TaskStatus",
				"",
				false,
				false,
				amqp.Publishing{
					ContentType: "application/json",
					Body:        byteEvent,
				})
			if err != nil {
				log.Printf("can`t publish message: %s", err.Error())
			}

			confirm := <-confirmation
			if confirm.Ack {
				log.Printf("published message: %s", byteEvent)
			} else {
				log.Printf("failed to publish a message %s, error: %s", byteEvent, err.Error())
			}

			//TODO: add CUD to analytics service

			w.Write([]byte("Ok!"))
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func ShuffleTasks(connection *pgx.Conn, channel *amqp.Channel, confirmation chan amqp.Confirmation) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {

			// get all popugs
			clientsRows, err := connection.Query(context.Background(), "SELECT id FROM clients")
			if err != nil {
				http.Error(w, ErrSomething.Error(), http.StatusInternalServerError)
			}
			clientsIDs := make([]string, 0)
			for clientsRows.Next() {
				var popugID string
				err = clientsRows.Scan(&popugID)
				if err != nil {
					log.Printf("error while parse: %s", err.Error())
				}
				clientsIDs = append(clientsIDs, popugID)
			}

			// start transaction
			updatedTasks := make([]TaskEntity, 0)
			tx, err := connection.BeginTx(context.Background(), pgx.TxOptions{})
			defer tx.Rollback(context.Background())
			tasksRows, err := tx.Query(context.Background(), "SELECT id FROM tasks where is_open = $1", true)
			if err != nil {
				http.Error(w, ErrSomething.Error(), http.StatusInternalServerError)
			}
			tasksIDs := make([]int, 0)
			for tasksRows.Next() {
				var taskID int
				if err = tasksRows.Scan(&taskID); err != nil {
					log.Printf("error while parse: %s", err.Error())
				}
				tasksIDs = append(tasksIDs, taskID)
			}

			for _, taskID := range tasksIDs {
				randomPopougID := rand.Intn(len(clientsIDs))
				var taskEntity TaskEntity
				row := tx.QueryRow(context.Background(), "UPDATE tasks SET popug_id = $1 WHERE id = $2 RETURNING popug_id, public_id", clientsIDs[randomPopougID], taskID)
				err = row.Scan(&taskEntity.PopugID, &taskEntity.PublicID)
				if err != nil {
					http.Error(w, ErrSomething.Error(), http.StatusInternalServerError)
				}
				updatedTasks = append(updatedTasks, taskEntity)
			}
			err = tx.Commit(context.Background())
			if err != nil {
				http.Error(w, ErrSomething.Error(), http.StatusInternalServerError)
			}

			for _, updatedTask := range updatedTasks {
				taskEvent := TaskEvent{
					PublicID: updatedTask.PublicID,
					PopugID:  updatedTask.PopugID,
				}

				byteEvent, err := json.Marshal(taskEvent)
				if err != nil {
					http.Error(w, ErrSomething.Error(), http.StatusInternalServerError)
				}

				err = channel.Publish(
					"trackerService.TaskStatus",
					"",
					false,
					false,
					amqp.Publishing{
						ContentType: "application/json",
						Body:        byteEvent,
					})
				if err != nil {
					log.Printf("can`t publish message: %s", err.Error())
				}
				confirm := <-confirmation
				if confirm.Ack {
					log.Printf("published message: %s", byteEvent)
				} else {
					log.Printf("failed to publish a message %s, error: %s", byteEvent, err.Error())
				}
			}

			//TODO: Add CUD to analytics service

		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func CloseTask(connection *pgx.Conn, channel *amqp.Channel, confirmation chan amqp.Confirmation) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		taskID := r.URL.Query().Get("task")
		rows := connection.QueryRow(context.Background(), "UPDATE tasks SET is_open = $1 where id = $2 RETURNING is_open, public_id, popug_id", false, taskID)
		var taskEntity TaskEntity
		err := rows.Scan(&taskEntity.IsOpen, &taskEntity.PublicID, &taskEntity.PopugID)
		if err != nil {
			http.Error(w, ErrSomething.Error(), http.StatusInternalServerError)
			return
		}

		taskEvent := TaskEvent{
			IsOpen:   taskEntity.IsOpen,
			PublicID: taskEntity.PublicID,
			PopugID:  taskEntity.PopugID,
		}

		byteEvent, err := json.Marshal(taskEvent)
		if err != nil {
			http.Error(w, ErrSomething.Error(), http.StatusInternalServerError)
		}
		log.Print(string(byteEvent))

		err = channel.Publish(
			"trackerService.TaskStatus",
			"",
			false,
			false,
			amqp.Publishing{
				ContentType: "application/json",
				Body:        byteEvent,
			})
		if err != nil {
			log.Printf("can`t publish message: %s", err.Error())
		}

		confirm := <-confirmation
		if confirm.Ack {
			log.Printf("published message: %s", byteEvent)
		} else {
			log.Printf("failed to publish a message %s, error: %s", byteEvent, err.Error())
		}

		//TODO: Add CUD to analytics service

		w.Write([]byte(fmt.Sprintf("Task '%s' successfully closed.", taskID)))

	}
}
