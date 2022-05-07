package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
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

func AuthHandler(client *http.Client) http.HandlerFunc {
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

func AddNewTaskHandler(connection *pgx.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Something goes wrong", http.StatusInternalServerError)
			}

			var task Task
			err = json.Unmarshal(body, &task)
			if err != nil {
				http.Error(w, "Something goes wrong", http.StatusInternalServerError)
			}

			_, err = connection.Exec(context.Background(), "INSERT INTO tasks (description, status, popug_id) VALUES ($1, $2, $3)", task.Description, task.Status, task.PopugID)
			if err != nil {
				log.Printf("can`t insert to DB: %s", err.Error())
			}

			//TODO: Send CUD and BE to account service and BE to analytics service

			w.Write([]byte("Ok!"))
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func ShuffleTasks() {}

func CloseTask() {}
