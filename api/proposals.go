package api

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/store"
)

type proposalBody struct {
	TeamID       int64  `json:"team_id"`
	Idea         string `json:"idea"`
	Plan         string `json:"plan"`
	Deadline     string `json:"deadline"`
	PrototypeURL string `json:"prototype_url"`
}

type proposalJSON struct {
	ID               int64  `json:"id"`
	TaskID           int64  `json:"task_id"`
	TeamID           int64  `json:"team_id"`
	TeamName         string `json:"team_name"`
	Idea             string `json:"idea"`
	Plan             string `json:"plan"`
	Deadline         string `json:"deadline"`
	PrototypeURL     string `json:"prototype_url"`
	Status           string `json:"status"`
	PointsAwarded    int    `json:"points_awarded"`
	CreatedAt        string `json:"created_at"`
	DecidedAt        string `json:"decided_at"`
	StageConfirmedAt string `json:"stage_confirmed_at"`
}

func proposalTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func viewProposal(p store.Proposal) proposalJSON {
	return proposalJSON{ID: p.ID, TaskID: p.TaskID, TeamID: p.TeamID, TeamName: p.TeamName,
		Idea: p.Idea, Plan: p.Plan, Deadline: p.Deadline, PrototypeURL: p.PrototypeURL,
		Status: p.Status, PointsAwarded: p.PointsAwarded, CreatedAt: proposalTime(p.CreatedAt),
		DecidedAt: proposalTime(p.DecidedAt), StageConfirmedAt: proposalTime(p.StageConfirmedAt)}
}

func proposalError(w http.ResponseWriter, err error) {
	var validation *store.ValidationError
	switch {
	case errors.As(err, &validation):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": validation.Message, "field": validation.Field})
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "Задача не найдена")
	default:
		log.Printf("предложения: %v", err)
		writeError(w, http.StatusInternalServerError, "Не удалось выполнить операцию с предложениями. Попробуйте позже.")
	}
}

// RegisterProposals exposes proposals without making any business decisions.
func RegisterProposals(mux *http.ServeMux, st *store.Store) {
	mux.HandleFunc("POST /api/tasks/{id}/proposals", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		var body proposalBody
		if !decodeFrontend(w, r, &body) {
			return
		}
		// Resolve an unknown task to 404 before store validation of publication.
		if _, err := st.GetTask(r.Context(), id); err != nil {
			proposalError(w, err)
			return
		}
		proposal, err := st.CreateProposal(r.Context(), store.ProposalInput{TaskID: id, TeamID: body.TeamID,
			Idea: body.Idea, Plan: body.Plan, Deadline: body.Deadline, PrototypeURL: body.PrototypeURL})
		if err != nil {
			proposalError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, viewProposal(proposal))
	})
	mux.HandleFunc("GET /api/tasks/{id}/proposals", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		if _, err := st.GetTask(r.Context(), id); err != nil {
			proposalError(w, err)
			return
		}
		proposals, err := st.ListProposalsByTask(r.Context(), id)
		if err != nil {
			proposalError(w, err)
			return
		}
		result := make([]proposalJSON, 0, len(proposals))
		for _, p := range proposals {
			result = append(result, viewProposal(p))
		}
		writeJSON(w, http.StatusOK, result)
	})
}
