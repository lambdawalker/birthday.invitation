package app

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
)

// NormalizePhone accepts formatting, not extensions or alphabetic dialing.
// Existing Mexican +521 mobile numbers share the current +52 identity.
func NormalizePhone(raw string) (string, error) {
	var b strings.Builder
	for _, c := range strings.TrimSpace(raw) {
		switch {
		case c >= '0' && c <= '9':
			b.WriteRune(c)
		case c == '+' && b.Len() == 0:
			b.WriteRune(c)
		case c == ' ' || c == '-' || c == '(' || c == ')' || c == '.':
		default:
			return "", errors.New("Use an international phone number without extensions")
		}
	}
	phone := b.String()
	if strings.HasPrefix(phone, "00") {
		phone = "+" + phone[2:]
	}
	if strings.HasPrefix(phone, "+521") && len(phone) == 14 {
		phone = "+52" + phone[4:]
	}
	if !phonePattern.MatchString(phone) {
		return "", errors.New("Use an international phone number, including + and country code")
	}
	return phone, nil
}

type ContactInput struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email,omitempty"`
}
type ContactImportResult struct {
	Index    int    `json:"index"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Status   string `json:"status"`
	FamilyID string `json:"familyId,omitempty"`
	Message  string `json:"message,omitempty"`
}

func (s *Server) importContacts(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.actor(r)
	if !ok {
		fail(w, 401, "Sign in before adding contacts")
		return
	}
	var body struct {
		Contacts []ContactInput `json:"contacts"`
	}
	if !decodeLimit(w, r, &body, 32768) {
		return
	}
	if len(body.Contacts) == 0 || len(body.Contacts) > 20 {
		fail(w, 400, "Choose between 1 and 20 contacts per request")
		return
	}
	families, err := s.store.List(r.Context())
	if err != nil {
		fail(w, 503, "Unable to check the existing roster. Try again.")
		return
	}
	existing := map[string]string{}
	for _, f := range families {
		if phone, e := NormalizePhone(f.Phone); e == nil {
			existing[phone] = f.ID
		}
	}
	results := make([]ContactImportResult, 0, len(body.Contacts))
	for i, c := range body.Contacts {
		c.Name = strings.TrimSpace(c.Name)
		c.Email = strings.TrimSpace(c.Email)
		phone, err := NormalizePhone(c.Phone)
		result := ContactImportResult{Index: i, Name: c.Name, Phone: phone, Status: "invalid"}
		switch {
		case c.Name == "" || len(c.Name) > 120:
			result.Message = "Enter a name up to 120 UTF-8 bytes"
		case err != nil:
			result.Message = err.Error()
		case len(c.Email) > 254:
			result.Message = "Email address is too long"
		default:
			if c.Email != "" {
				address, e := mail.ParseAddress(c.Email)
				if e != nil || address.Address != c.Email {
					result.Message = "Enter a valid email address"
					break
				}
			}
			if id, ok := existing[phone]; ok {
				result.Status = "existing"
				result.FamilyID = id
				break
			}
			f := Family{ID: RandomID(), Name: c.Name, Phone: phone, Email: c.Email, RSVP: "pending", Status: "pending", Revision: 1, UpdatedAt: now(),
				CreatedBy: actor.ID, CreatedName: actor.Name, AssignedTo: actor.ID, AssignedName: actor.Name,
				History: []Event{{ID: RandomID(), Status: "added", At: now(), Actor: actor.Name, Detail: "Added from phone contacts"}}}
			if actor.ID == "admin" {
				f.AssignedTo = ""
				f.AssignedName = ""
			}
			err = s.store.Create(r.Context(), f)
			if errors.Is(err, ErrConflict) {
				result.Status = "existing"
			} else if err != nil {
				result.Status = "error"
				result.Message = "Could not confirm this addition. Retry safely; existing numbers will be skipped."
			} else {
				result.Status = "added"
				result.FamilyID = f.ID
				existing[phone] = f.ID
			}
		}
		results = append(results, result)
	}
	send(w, 200, map[string]any{"results": results})
}
