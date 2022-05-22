package main

import (
	"context"

	"github.com/jackc/pgx/v4"
)

func RunMigrations() error {
	conn, _ := pgx.Connect(context.Background(), "postgres://postgres:postgres@localhost:5434/postgres")
	_, err := conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS clients (
	  id     	 bigserial PRIMARY KEY,
	  secret 	 TEXT  NOT NULL,
	  domain 	 TEXT  NOT NULL,
	  popug_id   TEXT  UNIQUE
	);`)
	if err != nil {
		return err
	}

	_, err = conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS accounts (
	  id 			bigserial PRIMARY KEY,
	  money		    INTEGER   NOT NULL,
	  popug_id 		TEXT 	  REFERENCES clients (popug_id) UNIQUE
	);`)
	if err != nil {
		return err
	}

	_, err = conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS logs (
	  id 			bigserial PRIMARY KEY,
	  money		    INTEGER   NOT NULL,
	  public_id		UUID 	  NOT NULL,
	  date			TIMESTAMP NOT NULL,
	  account_id 	INTEGER   REFERENCES accounts (id)
	);`)
	if err != nil {
		return err
	}

	return nil
}
