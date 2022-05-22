package main

import (
	"errors"
)

type UserEvent struct {
	ClientID     string `json:"ClientID"`
	ClientSecret string `json:"ClientSecret"`
	Role         string `json:"Role"`
}

var (
	ErrSomething     = errors.New("something goes wrong")
	ErrParseToken    = errors.New("error parse token")
	ErrInvalidSchema = errors.New("invalid event schema")
)
