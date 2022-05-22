package main

import (
	"context"

	"github.com/jackc/pgx/v4"
)

func RunMigrations() error {
	conn, _ := pgx.Connect(context.Background(), "postgres://postgres:postgres@localhost:5435/postgres")
	_, err := conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS clients (
	  id     	 TEXT  NOT NULL,
	  secret 	 TEXT  NOT NULL,
	  domain 	 TEXT  NOT NULL,
	  CONSTRAINT clients_pkey PRIMARY KEY (id)
	);`)
	if err != nil {
		return err
	}

	_, err = conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS closed_tasks (
	  id 			bigserial PRIMARY KEY,
	  money		    INTEGER   NOT NULL,
	  public_id		UUID 	  NOT NULL,
	  date			DATE	  NOT NULL
	);`)
	if err != nil {
		return err
	}

	_, err = conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS daily_money (
	  id 			bigserial PRIMARY KEY,
	  money		    INTEGER   NOT NULL,
	  popug_id		TEXT 	  NOT NULL,
	  date			DATE	  NOT NULL
	);`)
	if err != nil {
		return err
	}
	return nil
}
