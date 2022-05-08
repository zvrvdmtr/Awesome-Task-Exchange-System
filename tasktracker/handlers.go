package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"

	"github.com/jackc/pgx/v4"
)

type User struct {
	ClientID     string `json:"ClientID"`
	ClientSecret string `json:"ClientSecret"`
}

type Task struct {
	Description string `json:"Description"`
	Status      bool   `json:"Status"`
	PopugID     string `json:"PopugID"`
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
		rows, err := connection.Query(context.Background(), "SELECT id, description, status, popug_id FROM tasks where popug_id = $1", claims.StandardClaims.Audience)
		if err != nil {
			http.Error(w, ErrSomething.Error(), http.StatusInternalServerError)
		}
		tasks := make([]TaskEntity, 0)
		for rows.Next() {
			var task TaskEntity
			err = rows.Scan(&task.ID, &task.Description, &task.Status, &task.PopugID)
			if err != nil {
				log.Printf("error while parse: %s", err.Error())
			}
			tasks = append(tasks, task)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tasks)
	}
}

func AddTask(connection *pgx.Conn) http.HandlerFunc {
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

			_, err = connection.Exec(context.Background(), "INSERT INTO tasks (description, status, popug_id) VALUES ($1, $2, $3)", task.Description, task.Status, task.PopugID)
			if err != nil {
				log.Printf("can`t insert to DB: %s", err.Error())
			}

			//TODO: Send CUD and BE to account service and CUD to analytics service

			w.Write([]byte("Ok!"))
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func ShuffleTasks(connection *pgx.Conn) http.HandlerFunc {
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
			tx, err := connection.BeginTx(context.Background(), pgx.TxOptions{})
			defer tx.Rollback(context.Background())
			tasksRows, err := tx.Query(context.Background(), "SELECT id FROM tasks where status = $1", true)
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
			for taskID := range tasksIDs {
				randomPopougID := rand.Intn(100)
				_, err = tx.Exec(context.Background(), "UPDATE tasks SET popug_id = $1 WHERE id = $2", clientsIDs[randomPopougID], taskID)
				if err != nil {
					http.Error(w, ErrSomething.Error(), http.StatusInternalServerError)
				}
			}
			err = tx.Commit(context.Background())
			if err != nil {
				http.Error(w, ErrSomething.Error(), http.StatusInternalServerError)
			}

		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func CloseTask(connection *pgx.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		task := r.URL.Query().Get("task")
		_, err := connection.Exec(context.Background(), "UPDATE tasks SET status = $1 where id = $2", false, task)
		if err != nil {
			http.Error(w, ErrSomething.Error(), http.StatusInternalServerError)
			return
		}

		//TODO: Send BE to account service and CUD to analytics service

		w.Write([]byte(fmt.Sprintf("Task '%s' successfully closed.", task)))

	}
}
