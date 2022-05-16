package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/streadway/amqp"
)

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

func PopugAccount(conn *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var account AccountEntity
		row := conn.QueryRow(context.Background(), "SELECT id, money FROM accounts WHERE popug_id = $1", "Leha17")
		err := row.Scan(&account.ID, &account.Money)
		if err != nil {
			log.Printf("Can't scan row %s", err.Error())
		}

		currentTime := time.Now()
		rows, err := conn.Query(
			context.Background(),
			"SELECT id, money, public_id, date, account_id FROM logs WHERE account_id = $1 AND date > $2",
			account.ID,
			currentTime.Format("01-02-2006"),
		)

		if err != nil {
			http.Error(w, ErrSomething.Error(), http.StatusInternalServerError)
		}
		logRecords := make([]LogRecordEntity, 0)
		for rows.Next() {
			var logRecord LogRecordEntity
			err = rows.Scan(&logRecord.ID, &logRecord.Money, &logRecord.PublicID, &logRecord.Date, &logRecord.AccountID)
			if err != nil {
				log.Printf("Can't scan row %s", err.Error())
			}

			logRecords = append(logRecords, logRecord)
		}

		w.Header().Set("Content-Type", "Application/JSON")

		json.NewEncoder(w).Encode(map[string]interface{}{
			"ID":      account.ID,
			"Money":   account.Money,
			"Actions": logRecords,
		})
	}
}

func AllAccounts(conn *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountWithLogs := make([]AccountWithLogs, 0)
		rows, err := conn.Query(context.Background(), "SELECT id, money FROM accounts")
		for rows.Next() {
			var account AccountEntity
			err = rows.Scan(&account.ID, &account.Money)
			if err != nil {
				log.Printf("Can't scan row %s", err.Error())
			}

			currentTime := time.Now()
			logRows, err := conn.Query(
				context.Background(),
				"SELECT id, money, public_id, date, account_id FROM logs WHERE account_id = $1 AND date > $2",
				account.ID,
				currentTime.Format("01-02-2006"),
			)
			if err != nil {
				log.Printf("Can't query rows %s", err.Error())
			}
			logRecords := make([]LogRecordEntity, 0)
			for logRows.Next() {
				var logRecord LogRecordEntity
				err = logRows.Scan(&logRecord.ID, &logRecord.Money, &logRecord.PublicID, &logRecord.Date, &logRecord.AccountID)
				if err != nil {
					log.Printf("Can't scan row %s", err.Error())
				}
				logRecords = append(logRecords, logRecord)
			}
			accountWithLogs = append(accountWithLogs, AccountWithLogs{
				account,
				logRecords,
			})
		}

		w.Header().Set("Content-Type", "Application/JSON")
		json.NewEncoder(w).Encode(accountWithLogs)
	}
}

// DailyResult handler call only by scheduler
func DailyResult(conn *pgxpool.Pool, channel *amqp.Channel) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		currentTime := time.Now()
		yesterday := currentTime.AddDate(0, 0, -1)
		logRows, err := conn.Query(
			context.Background(),
			"SELECT account_id, sum(money) FROM logs WHERE date > $1 AND date < $2 group by account_id",
			yesterday.Format("2006-01-02"),
			currentTime.Format("2006-01-02"),
		)

		if err != nil {
			log.Printf("Can't query rows %s", err.Error())
		}
		for logRows.Next() {
			var Money int
			var AccountID int
			var PopugID string
			err = logRows.Scan(&AccountID, &Money)
			if err != nil {
				log.Printf("Can't scan row %s", err)
			}

			if Money > 0 {
				row := conn.QueryRow(
					context.Background(),
					"UPDATE accounts SET money=money-$1 WHERE id = $2 RETURNING popug_id",
					Money,
					AccountID,
				)
				err = row.Scan(&PopugID)
				if err != nil {
					log.Printf("Can't scan row %s", err)
				}
				//TODO send email somewhere here

			} else {
				accountRow := conn.QueryRow(context.Background(), "SELECT popug_id FROM accounts WHERE id = $1", AccountID)
				err = accountRow.Scan(&PopugID)
				if err != nil {
					log.Printf("Can't scan row %s", err)
				}
			}
			byteMoneyEvent, err := json.Marshal(DailyMoneyEvent{
				Money:   Money,
				PopugID: PopugID,
				Date:    time.Now(),
			})
			if err != nil {
				log.Printf("can`t marshal struct to body: %s", err.Error())
			}

			channel.Publish(
				"accountingService.DailyMoney",
				"",
				false,
				false,
				amqp.Publishing{
					ContentType: "application/json",
					Body:        byteMoneyEvent,
				})
		}
	}
}
