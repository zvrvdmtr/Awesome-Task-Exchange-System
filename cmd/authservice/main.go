package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/go-oauth2/oauth2/v4/errors"
	"github.com/go-oauth2/oauth2/v4/generates"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/models"
	"github.com/go-oauth2/oauth2/v4/server"
	"github.com/go-oauth2/oauth2/v4/store"
	"github.com/golang-jwt/jwt"
	"github.com/jackc/pgx/v4"
	pg "github.com/vgarvardt/go-oauth2-pg/v4"
	"github.com/vgarvardt/go-pg-adapter/pgx4adapter"
)

type User struct {
	ClientID     string `json:"ClientID"`
	ClientSecret string `json:"ClientSecret"`
	Role         string `json:"Role"`
}

func main() {
	conn, _ := pgx.Connect(context.Background(), "postgres://postgres:postgres@localhost:5432/postgres")

	manager := manage.NewDefaultManager()
	manager.SetAuthorizeCodeTokenCfg(manage.DefaultAuthorizeCodeTokenCfg)

	// token memory store
	manager.MustTokenStorage(store.NewMemoryTokenStore())

	//client pg store
	adapter := pgx4adapter.NewConn(conn)
	clientStore, _ := pg.NewClientStore(adapter)

	manager.MapClientStorage(clientStore)
	manager.MapAccessGenerate(generates.NewJWTAccessGenerate("", []byte("00000000"), jwt.SigningMethodHS512))

	srv := server.NewDefaultServer(manager)
	srv.SetAllowGetAccessRequest(true)
	srv.SetClientInfoHandler(server.ClientFormHandler)

	srv.SetInternalErrorHandler(func(err error) (re *errors.Response) {
		log.Printf("Internal Error: %s", err.Error())
		return
	})

	srv.SetResponseErrorHandler(func(re *errors.Response) {
		log.Printf("Response Error: %s", re.Error.Error())
	})

	http.HandleFunc("/registration", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				log.Printf("can`t read body: %s", err.Error())
			}

			var newUser User
			err = json.Unmarshal(body, &newUser)
			if err != nil {
				log.Printf("can`t unmarshal body to struct: %s", err.Error())
			}

			err = clientStore.Create(&models.Client{
				ID:     newUser.ClientID,
				Secret: newUser.ClientSecret,
				Domain: newUser.Role, // use Domain field as Role field
			})
			if err != nil {
				log.Printf("can`t add new user: %s", err.Error())
			}

			w.Header().Set("Content-Type", "application/json")

			// TODO: Send CUD event to RabbitMQ

			json.NewEncoder(w).Encode(map[string]string{
				"CLIENT_ID":     newUser.ClientID,
				"CLIENT_SECRET": newUser.ClientSecret,
				"ROLE":          newUser.Role,
			})
		}
	})

	http.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		srv.HandleTokenRequest(w, r)
	})

	http.HandleFunc("/verify", func(w http.ResponseWriter, r *http.Request) {
		_, err := srv.ValidationBearerToken(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	})

	log.Fatal(http.ListenAndServe(":9096", nil))
}
