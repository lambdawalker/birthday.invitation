package app

import (
	"net/http"
	"strings"
)

type Action struct {
	Action    string `json:"action"`
	Channel   string `json:"channel"`
	AttemptID string `json:"attemptId"`
	Status    string `json:"status"`
	Detail    string `json:"detail"`
}

func (s *Server) action(w http.ResponseWriter, r *http.Request) {
	d, ok := s.actor(r)
	if !ok {
		fail(w, 401, "Acceso no autorizado")
		return
	}
	var b Action
	if !decode(w, r, &b) {
		return
	}
	if len(b.Detail) > 200 || len(b.AttemptID) > 100 {
		fail(w, 400, "Datos demasiado largos")
		return
	}
	f, ok := s.family(w, r)
	if !ok {
		return
	}
	rev := f.Revision
	if b.Action == "claim" {
		if f.AssignedTo != "" && f.AssignedTo != d.ID {
			fail(w, 409, "Familia asignada a otro dispositivo")
			return
		}
		f.AssignedTo = d.ID
		f.AssignedName = d.Name
	} else {
		if d.ID != "admin" && f.AssignedTo != d.ID {
			fail(w, 409, "Primero asigna esta familia a tu dispositivo")
			return
		}
		if d.ID == "admin" && f.AssignedTo != "" && f.AssignedTo != "admin" && b.Action != "release" && b.Action != "result" {
			fail(w, 409, "Libera la asignación antes de editar manualmente")
			return
		}
		switch b.Action {
		case "release":
			if f.Status == "sending" {
				fail(w, 409, "Resuelve el envío en curso antes de liberar")
				return
			}
			f.AssignedTo = ""
			f.AssignedName = ""
		case "begin":
			if b.AttemptID == "" || (b.Channel != "sms" && b.Channel != "whatsapp") {
				fail(w, 400, "Falta intento o canal")
				return
			}
			// Repeating begin acknowledges only the same attempt. Client must never infer permission to resend from this response.
			if f.AttemptID == b.AttemptID {
				send(w, 200, f)
				return
			}
			if f.Status != "pending" && f.Status != "failed" {
				fail(w, 409, "Ya hay un envío registrado; revisa antes de reenviar")
				return
			}
			for _, e := range f.History {
				if e.ID == b.AttemptID {
					fail(w, 409, "Intento usado anteriormente")
					return
				}
			}
			f.AttemptID = b.AttemptID
			f.Channel = b.Channel
			f.Status = "sending"
			if b.Channel == "whatsapp" {
				f.Status = "whatsapp_opened"
			}
			f.History = append(f.History, Event{b.AttemptID, f.Status, f.Channel, now(), d.Name, ""})
		case "result":
			if b.AttemptID == "" || b.AttemptID != f.AttemptID {
				fail(w, 409, "Intento no coincide")
				return
			}
			allowed := b.Status == "uncertain" || b.Status == "failed" || (f.Channel == "sms" && (b.Status == "sent" || b.Status == "delivered")) || ((f.Channel == "whatsapp" || f.Channel == "sms") && b.Status == "manual_sent")
			if !allowed {
				fail(w, 400, "Estado inválido para el canal")
				return
			}
			if f.Status == b.Status {
				send(w, 200, f)
				return
			}
			if f.Status == "delivered" || (f.Status == "sent" && b.Status != "delivered") || f.Status == "manual_sent" {
				fail(w, 409, "No se puede degradar un envío confirmado")
				return
			}
			f.Status = b.Status
			f.History = append(f.History, Event{b.AttemptID, b.Status, f.Channel, now(), d.Name, b.Detail})
		case "manual":
			if d.ID != "admin" {
				fail(w, 403, "Solo administrador")
				return
			}
			if b.Channel != "sms" && b.Channel != "whatsapp" {
				fail(w, 400, "Canal inválido")
				return
			}
			if f.Status == "sending" {
				fail(w, 409, "Envío en curso")
				return
			}
			f.Status = "manual_sent"
			f.Channel = b.Channel
			f.History = append(f.History, Event{RandomID(), f.Status, b.Channel, now(), d.Name, "Confirmado manualmente"})
		case "reset":
			if d.ID != "admin" {
				fail(w, 403, "Solo administrador")
				return
			}
			if strings.TrimSpace(b.Detail) == "" {
				fail(w, 400, "Indica el motivo de reenvío")
				return
			}
			if f.Status == "sending" {
				fail(w, 409, "Envío en curso")
				return
			}
			f.Status = "pending"
			f.AttemptID = ""
			f.History = append(f.History, Event{RandomID(), "pending", f.Channel, now(), d.Name, b.Detail})
		default:
			fail(w, 400, "Acción desconocida")
			return
		}
	}
	f.Revision++
	f.UpdatedAt = now()
	if s.save(w, r, f, rev) {
		send(w, 200, f)
	}
}
