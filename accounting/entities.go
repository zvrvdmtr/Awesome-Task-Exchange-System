package main

import (
	"time"

	"github.com/google/uuid"
)

type UserEvent struct {
	ClientID     string `json:"ClientID"`
	ClientSecret string `json:"ClientSecret"`
	Role         string `json:"Role"`
}

type TaskEvent struct {
	JiraID      string    `json:"jira_id"`
	Description string    `json:"description"`
	IsOpen      bool      `json:"is_open"`
	PopugID     string    `json:"popug_id"`
	PublicID    uuid.UUID `json:"public_id"`
}

type TaskWithMoneyAndDateEvent struct {
	PopugID  string
	PublicID uuid.UUID
	Money    int
	Date     time.Time
}

type DailyMoneyEvent struct {
	PopugID string
	Money   int
	Date    time.Time
}

type User struct {
	ClientID     string `json:"ClientID"`
	ClientSecret string `json:"ClientSecret"`
}

type LogRecordEntity struct {
	ID        int
	Money     int
	PublicID  uuid.UUID
	Date      time.Time
	AccountID int
}

type AccountEntity struct {
	ID      int
	Money   int
	PopugID int
}

type AccountWithLogs struct {
	Account AccountEntity
	Logs    []LogRecordEntity
}
