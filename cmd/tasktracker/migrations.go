package main

import (
	"context"

	"github.com/jackc/pgx/v4"
)

func init() {
	conn, _ := pgx.Connect(context.Background(), "postgres://postgres:postgres@localhost:5433/postgres")
	conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS clients (
	  id     TEXT  NOT NULL,
	  secret TEXT  NOT NULL,
	  domain TEXT  NOT NULL,
	  data   JSONB NOT NULL,
	  CONSTRAINT clients_pkey PRIMARY KEY (id)
	);`)
}
