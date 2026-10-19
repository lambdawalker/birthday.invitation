package app

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/mail"
	"regexp"
	"strings"
)

var phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

func ReadGuests(r io.Reader, namespace string) ([]Family, error) {
	if len(namespace) < 32 {
		return nil, fmt.Errorf("private import namespace must be at least 32 characters")
	}
	rows, e := csv.NewReader(r).ReadAll()
	if e != nil {
		return nil, e
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("CSV requires headers and guests")
	}
	cols := map[string]int{}
	for i, h := range rows[0] {
		cols[strings.TrimSpace(strings.TrimPrefix(h, "\ufeff"))] = i
	}
	for _, c := range []string{"name", "phone", "email"} {
		if _, ok := cols[c]; !ok {
			return nil, fmt.Errorf("missing column %s", c)
		}
	}
	get := func(row []string, k string) string {
		i, ok := cols[k]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}
	fs := []Family{}
	seen := map[string]bool{}
	for i, row := range rows[1:] {
		name, phone, email := get(row, "name"), get(row, "phone"), get(row, "email")
		if name == "" || len(name) > 120 || !phonePattern.MatchString(phone) {
			return nil, fmt.Errorf("row %d: name and international phone (+52...) required", i+2)
		}
		if email != "" {
			a, e := mail.ParseAddress(email)
			if e != nil || a.Address != email {
				return nil, fmt.Errorf("row %d: invalid email", i+2)
			}
		}
		id := get(row, "id")
		if id == "" {
			id = HashToken(namespace + ":" + phone)[:32]
		}
		if !idPattern.MatchString(id) || seen[id] {
			return nil, fmt.Errorf("row %d: invalid or duplicate id/phone", i+2)
		}
		seen[id] = true
		fs = append(fs, Family{ID: id, Name: name, Phone: phone, Email: email, RSVP: "pending", Status: "pending", Revision: 1, UpdatedAt: now()})
	}
	return fs, nil
}
