package main

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-oauth2/oauth2/v4/generates"
	"github.com/golang-jwt/jwt"
	"github.com/jackc/pgx/v4"
)

var (
	ErrSomething  = errors.New("something goes wrong")
	ErrParseToken = errors.New("error parse token")
)

type UserDBItem struct {
	ID     string `db:"id"`
	Secret string `db:"secret"`
	Domain string `db:"domain"`
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
