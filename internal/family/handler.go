package family

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/PhonIXQ/daily_expenses-backend/internal/auth"
	"github.com/PhonIXQ/daily_expenses-backend/internal/platform/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register adds routes to an authenticated mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("POST /v1/families", httpx.HandlerFunc(h.create))
	mux.Handle("GET /v1/families", httpx.HandlerFunc(h.list))
	mux.Handle("GET /v1/families/{familyID}", httpx.HandlerFunc(h.get))
	mux.Handle("PATCH /v1/families/{familyID}", httpx.HandlerFunc(h.update))

	mux.Handle("GET /v1/families/{familyID}/members", httpx.HandlerFunc(h.listMembers))
	mux.Handle("POST /v1/families/{familyID}/members", httpx.HandlerFunc(h.addMember))
	mux.Handle("PATCH /v1/families/{familyID}/members/{memberID}", httpx.HandlerFunc(h.updateMember))
	mux.Handle("DELETE /v1/families/{familyID}/members/{memberID}", httpx.HandlerFunc(h.removeMember))

	mux.Handle("POST /v1/families/{familyID}/invites", httpx.HandlerFunc(h.createInvite))
	mux.Handle("POST /v1/invites/{code}/accept", httpx.HandlerFunc(h.acceptInvite))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	var in CreateFamilyInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	out, err := h.svc.Create(r.Context(), auth.UserID(r.Context()), in)
	return respond(w, http.StatusCreated, out, err)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	out, err := h.svc.List(r.Context(), auth.UserID(r.Context()))
	return respond(w, http.StatusOK, out, err)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) error {
	familyID, err := pathID(r, "familyID")
	if err != nil {
		return err
	}
	out, err := h.svc.Get(r.Context(), auth.UserID(r.Context()), familyID)
	return respond(w, http.StatusOK, out, err)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	familyID, err := pathID(r, "familyID")
	if err != nil {
		return err
	}
	var in UpdateFamilyInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	out, err := h.svc.Update(r.Context(), auth.UserID(r.Context()), familyID, in)
	return respond(w, http.StatusOK, out, err)
}

func (h *Handler) listMembers(w http.ResponseWriter, r *http.Request) error {
	familyID, err := pathID(r, "familyID")
	if err != nil {
		return err
	}
	out, err := h.svc.ListMembers(r.Context(), auth.UserID(r.Context()), familyID)
	return respond(w, http.StatusOK, out, err)
}

func (h *Handler) addMember(w http.ResponseWriter, r *http.Request) error {
	familyID, err := pathID(r, "familyID")
	if err != nil {
		return err
	}
	var in NewMember
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	out, err := h.svc.AddMember(r.Context(), auth.UserID(r.Context()), familyID, in)
	return respond(w, http.StatusCreated, out, err)
}

func (h *Handler) updateMember(w http.ResponseWriter, r *http.Request) error {
	familyID, err := pathID(r, "familyID")
	if err != nil {
		return err
	}
	memberID, err := pathID(r, "memberID")
	if err != nil {
		return err
	}
	var in UpdateMemberInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	out, err := h.svc.UpdateMember(r.Context(), auth.UserID(r.Context()), familyID, memberID, in)
	return respond(w, http.StatusOK, out, err)
}

func (h *Handler) removeMember(w http.ResponseWriter, r *http.Request) error {
	familyID, err := pathID(r, "familyID")
	if err != nil {
		return err
	}
	memberID, err := pathID(r, "memberID")
	if err != nil {
		return err
	}
	err = h.svc.RemoveMember(r.Context(), auth.UserID(r.Context()), familyID, memberID)
	return respond(w, http.StatusNoContent, nil, err)
}

func (h *Handler) createInvite(w http.ResponseWriter, r *http.Request) error {
	familyID, err := pathID(r, "familyID")
	if err != nil {
		return err
	}
	var in CreateInviteInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	out, err := h.svc.CreateInvite(r.Context(), auth.UserID(r.Context()), familyID, in)
	return respond(w, http.StatusCreated, out, err)
}

func (h *Handler) acceptInvite(w http.ResponseWriter, r *http.Request) error {
	var in AcceptInviteInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	out, err := h.svc.AcceptInvite(r.Context(), auth.UserID(r.Context()), r.PathValue("code"), in)
	return respond(w, http.StatusOK, out, err)
}

func respond(w http.ResponseWriter, status int, v any, err error) error {
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, status, v)
	return nil
}

func pathID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		return uuid.Nil, httpx.NotFound(name + " not found")
	}
	return id, nil
}
