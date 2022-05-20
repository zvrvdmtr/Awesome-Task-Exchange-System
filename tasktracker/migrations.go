package main

import (
	"context"
	"log"

	"github.com/jackc/pgx/v4"
)

func RunMigrations() error {
	conn, _ := pgx.Connect(context.Background(), "postgres://postgres:postgres@localhost:5433/postgres")
	_, err := conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS clients (
	  id     	 TEXT  NOT NULL,
	  secret 	 TEXT  NOT NULL,
	  domain 	 TEXT  NOT NULL,
	  CONSTRAINT clients_pkey PRIMARY KEY (id)
	);`)
	if err != nil {
		log.Fatalf("Migration failed %s", err.Error())
	}

	_, err = conn.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS tasks (
	  id 			bigserial PRIMARY KEY,
	  description   TEXT 	  NOT NULL,
	  is_open		bool 	  NOT NULL,
	  public_id		UUID 	  NOT NULL,
	  popug_id 		TEXT 	  REFERENCES clients (id)
	);`)
	if err != nil {
		log.Fatalf("Migration failed %s", err.Error())
	}

	// Add new field to DB
	//_, err = conn.Exec(context.Background(), `
	//ALTER TABLE  tasks ADD COLUMN jira_id TEXT`)
	//if err != nil {
	//	log.Fatalf("Migration failed %s", err.Error())
	//}

	return nil
}
