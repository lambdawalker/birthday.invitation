package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

var ErrConflict = errors.New("conflict")
var ErrNotFound = errors.New("not found")

type Event struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Channel string `json:"channel"`
	At      string `json:"at"`
	Actor   string `json:"actor"`
	Detail  string `json:"detail,omitempty"`
}
type Family struct {
	CreatedBy    string  `json:"createdBy,omitempty"`
	CreatedName  string  `json:"createdName,omitempty"`
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Phone        string  `json:"phone"`
	Email        string  `json:"email,omitempty"`
	RSVP         string  `json:"rsvp"`
	Adults       int     `json:"adults"`
	Children     int     `json:"children"`
	Status       string  `json:"status"`
	Channel      string  `json:"channel,omitempty"`
	AssignedTo   string  `json:"assignedTo,omitempty"`
	AssignedName string  `json:"assignedName,omitempty"`
	AttemptID    string  `json:"attemptId,omitempty"`
	UpdatedAt    string  `json:"updatedAt,omitempty"`
	Revision     int     `json:"revision"`
	History      []Event `json:"history"`
}
type Device struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	Revoked   bool   `json:"revoked"`
}

func RandomID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}
func now() string { return time.Now().UTC().Format(time.RFC3339) }
