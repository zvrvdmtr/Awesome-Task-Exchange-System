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
	Money    int
	Date     time.Time
	PublicID uuid.UUID
}

type DailyMoneyEvent struct {
	Money   int
	Date    time.Time
	PopugID string
}
