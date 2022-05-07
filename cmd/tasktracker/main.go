package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-oauth2/oauth2/v4/errors"
	"github.com/go-oauth2/oauth2/v4/generates"
	"github.com/golang-jwt/jwt"
	"github.com/jackc/pgx/v4"
)

type User struct {
	ClientID     string `json:"ClientID"`
	ClientSecret string `json:"ClientSecret"`
}

type UserDBItem struct {
	ID     string `db:"id"`
	Secret string `db:"secret"`
	Domain string `db:"domain"`
}

var (
	ErrSomething  = errors.New("Something goes wrong")
	ErrParseToken = errors.New("Error parse token")
)

func main() {
	conn, _ := pgx.Connect(context.Background(), "postgres://postgres:postgres@localhost:5433/postgres")
	client := http.Client{Timeout: 1 * time.Second}
	http.HandleFunc("/protected", isAdminMiddleware(validateToken(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, I'm protected"))
	}, &client), conn))

	http.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
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
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func isAdminMiddleware(f http.HandlerFunc, conn *pgx.Conn) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		access := r.Header.Get("Authorization")
		token, err := jwt.ParseWithClaims(strings.Split(access, " ")[1], &generates.JWTAccessClaims{}, func(t *jwt.Token) (interface{}, error) {
			return []byte("00000000"), nil
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		claims, ok := token.Claims.(*generates.JWTAccessClaims)
		if !ok {
			http.Error(w, ErrParseToken.Error(), http.StatusBadRequest)
			return
		}

		var userItem UserDBItem
		row := conn.QueryRow(context.Background(), `SELECT id, secret, domain FROM clients where ID = $1`, claims.StandardClaims.Audience)
		err = row.Scan(&userItem.ID, &userItem.Secret, &userItem.Domain)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if userItem.Domain != "Admin" {
			http.Error(w, "access denied", http.StatusBadRequest)
			return
		}
		f.ServeHTTP(w, r)
	})
}

func validateToken(f http.HandlerFunc, client *http.Client) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		resp, err := client.Get(fmt.Sprintf("http://localhost:9096/verify?access_token=%s", strings.Split(token, " ")[1]))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if resp.StatusCode == http.StatusBadRequest {
			http.Error(w, "access denied", http.StatusBadRequest)
			return
		}
		f.ServeHTTP(w, r)
	})
}
