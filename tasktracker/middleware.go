package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-oauth2/oauth2/v4/generates"
	"github.com/golang-jwt/jwt"
	"github.com/jackc/pgx/v4"
)

type UserDBItem struct {
	ID     string `db:"id"`
	Secret string `db:"secret"`
	Domain string `db:"domain"`
}

type TaskEntity struct {
	ID          int    `db:"id"`
	Description string `db:"description"`
	Status      bool   `db:"status"`
	PopugID     string `db:"popug_id"`
}

func parseToken(access string) (*generates.JWTAccessClaims, error) {
	token, err := jwt.ParseWithClaims(strings.Split(access, " ")[1], &generates.JWTAccessClaims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte("00000000"), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*generates.JWTAccessClaims)
	if !ok {
		return nil, err
	}

	return claims, nil
}

func IsAdminMiddleware(f http.HandlerFunc, conn *pgx.Conn) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		access := r.Header.Get("Authorization")
		claims, err := parseToken(access)
		if err != nil {
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

func ValidateTokenMiddleware(f http.HandlerFunc, client *http.Client) http.HandlerFunc {
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

func CurrentUserMiddleware(f http.HandlerFunc, conn *pgx.Conn) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		access := r.Header.Get("Authorization")
		claims, err := parseToken(access)
		if err != nil {
			http.Error(w, ErrParseToken.Error(), http.StatusBadRequest)
			return
		}
		var task TaskEntity
		taskID, err := strconv.Atoi(r.URL.Query().Get("task"))
		fmt.Println(claims.StandardClaims.Audience)
		row := conn.QueryRow(
			context.Background(),
			`SELECT id, description, status, popug_id FROM tasks where popug_id = $1 and id = $2 and status = $3`,
			claims.StandardClaims.Audience,
			taskID,
			true)
		err = row.Scan(&task.ID, &task.Description, &task.Status, &task.PopugID)
		if err != nil {
			http.Error(w, "Access denied", http.StatusBadRequest)
			return
		}

		f.ServeHTTP(w, r)
	})
}
