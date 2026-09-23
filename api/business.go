package api

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/rating"
	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/store"
)

type businessTaskJSON struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	Company    string `json:"company"`
	Industry   string `json:"industry"`
	Category   string `json:"category"`
	Status     string `json:"status"`
	Score      int    `json:"score"`
	Level      string `json:"level"`
	LevelLabel string `json:"level_label"`
	Proposals  int    `json:"proposals"`
	Pending    int    `json:"pending"`
	Accepted   int    `json:"accepted"`
	Rejected   int    `json:"rejected"`
}

type businessTeamJSON struct {
	Name      string   `json:"name"`
	Interests []string `json:"interests"`
	Skills    string   `json:"skills"`
	Tech      string   `json:"tech"`
	Points    int      `json:"points"`
}

type fullProposalJSON struct {
	proposalJSON
	Team businessTeamJSON `json:"team"`
}

func businessError(w http.ResponseWriter, err error, conflict string) {
	var validation *store.ValidationError
	switch {
	case errors.As(err, &validation):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": validation.Message, "field": validation.Field})
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "Запись не найдена")
	case errors.Is(err, store.ErrInvalidTransition):
		writeError(w, http.StatusConflict, conflict)
	default:
		log.Printf("business API: %v", err)
		writeError(w, http.StatusInternalServerError, "Не удалось выполнить операцию. Попробуйте позже.")
	}
}

func decodeBusiness(w http.ResponseWriter, r *http.Request, body any) bool {
	if r.Body == nil || r.Body == http.NoBody || r.ContentLength == 0 {
		writeError(w, http.StatusBadRequest, "Укажите тело запроса в формате JSON")
		return false
	}
	return decodeFrontend(w, r, body)
}

// The store's single-proposal getter is private. Resolve only its task ID here,
// then reuse the public list method and the existing proposal JSON mapping.
func readBusinessProposal(ctx context.Context, st *store.Store, id int64) (store.Proposal, error) {
	var taskID int64
	if err := st.DB.QueryRowContext(ctx, `SELECT task_id FROM proposals WHERE id = ?`, id).Scan(&taskID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.Proposal{}, store.ErrNotFound
		}
		return store.Proposal{}, err
	}
	proposals, err := st.ListProposalsByTask(ctx, taskID)
	if err != nil {
		return store.Proposal{}, err
	}
	for _, proposal := range proposals {
		if proposal.ID == id {
			return proposal, nil
		}
	}
	return store.Proposal{}, store.ErrNotFound
}

func RegisterBusiness(mux *http.ServeMux, st *store.Store) {
	mux.HandleFunc("GET /api/business/companies", func(w http.ResponseWriter, r *http.Request) {
		companies, err := st.ListCompanies(r.Context())
		if err != nil {
			businessError(w, err, "")
			return
		}
		writeJSON(w, http.StatusOK, companies)
	})
	mux.HandleFunc("GET /api/business/tasks", func(w http.ResponseWriter, r *http.Request) {
		tasks, err := st.ListBusinessTasks(r.Context(), r.URL.Query().Get("company"))
		if err != nil {
			businessError(w, err, "")
			return
		}
		result := make([]businessTaskJSON, 0, len(tasks))
		for _, task := range tasks {
			level := rating.LevelFor(task.Score)
			result = append(result, businessTaskJSON{ID: task.ID, Title: task.Title, Company: task.Company, Industry: task.Industry,
				Category: task.Category, Status: task.Status, Score: task.Score, Level: level, LevelLabel: rating.LevelLabel(level),
				Proposals: task.Proposals, Pending: task.Pending, Accepted: task.Accepted, Rejected: task.Rejected})
		}
		writeJSON(w, http.StatusOK, result)
	})
	mux.HandleFunc("GET /api/tasks/{id}/proposals/full", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		if _, err := st.GetTask(r.Context(), id); err != nil {
			businessError(w, err, "")
			return
		}
		proposals, err := st.ListProposalsByTask(r.Context(), id)
		if err != nil {
			businessError(w, err, "")
			return
		}
		result := make([]fullProposalJSON, 0, len(proposals))
		teams := map[int64]businessTeamJSON{}
		for _, proposal := range proposals {
			team, ok := teams[proposal.TeamID]
			if !ok {
				t, err := st.GetTeam(r.Context(), proposal.TeamID)
				if err != nil {
					businessError(w, err, "")
					return
				}
				interests := t.Interests
				if interests == nil {
					interests = []string{}
				}
				team = businessTeamJSON{Name: t.Name, Interests: interests, Skills: t.Skills, Tech: t.Tech, Points: t.Points}
				teams[proposal.TeamID] = team
			}
			result = append(result, fullProposalJSON{proposalJSON: viewProposal(proposal), Team: team})
		}
		writeJSON(w, http.StatusOK, result)
	})
	mux.HandleFunc("POST /api/proposals/{id}/decision", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		var body struct {
			Status string `json:"status"`
		}
		if !decodeBusiness(w, r, &body) {
			return
		}
		if body.Status != store.ProposalAccepted && body.Status != store.ProposalRejected {
			writeError(w, http.StatusBadRequest, "Выберите: принять или отклонить")
			return
		}
		if err := st.DecideProposal(r.Context(), id, body.Status); err != nil {
			businessError(w, err, "Решение по этому предложению уже принято")
			return
		}
		proposal, err := readBusinessProposal(r.Context(), st, id)
		if err != nil {
			businessError(w, err, "")
			return
		}
		writeJSON(w, http.StatusOK, viewProposal(proposal))
	})
	mux.HandleFunc("POST /api/proposals/{id}/stage", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		var body struct {
			Points int `json:"points"`
		}
		if !decodeBusiness(w, r, &body) {
			return
		}
		if body.Points < 1 || body.Points > 100 {
			writeError(w, http.StatusBadRequest, "Баллы за этап: от 1 до 100")
			return
		}
		if err := st.ConfirmStage(r.Context(), id, body.Points); err != nil {
			businessError(w, err, "Этап можно подтвердить только для принятого предложения, один раз")
			return
		}
		proposal, err := readBusinessProposal(r.Context(), st, id)
		if err != nil {
			businessError(w, err, "")
			return
		}
		writeJSON(w, http.StatusOK, viewProposal(proposal))
	})
}
