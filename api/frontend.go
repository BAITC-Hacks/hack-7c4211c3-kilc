package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/rating"
	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/store"
)

// Frontend routes expose the existing store. Role/team selection is a demo
// preference, not authentication; this application has no user accounts.
func RegisterFrontend(mux *http.ServeMux, st *store.Store) {
	h := frontendHandler{st}
	mux.HandleFunc("POST /api/rating", Rating)
	mux.HandleFunc("GET /api/categories", func(w http.ResponseWriter, r *http.Request) {
		items := make([]map[string]string, 0, len(store.Categories))
		for _, c := range store.Categories {
			items = append(items, map[string]string{"code": c.Code, "label": c.Label})
		}
		writeJSON(w, http.StatusOK, items)
	})
	mux.HandleFunc("GET /api/teams", h.listTeams)
	mux.HandleFunc("POST /api/teams", h.createTeam)
	mux.HandleFunc("GET /api/tasks", h.catalog)
	mux.HandleFunc("POST /api/tasks", h.createTask)
	mux.HandleFunc("GET /api/tasks/{id}", h.getTask)
	mux.HandleFunc("PUT /api/tasks/{id}", h.updateTask)
	mux.HandleFunc("POST /api/tasks/{id}/publish", h.publishTask)
}

type frontendHandler struct{ st *store.Store }

type teamBody struct {
	Name      string   `json:"name"`
	Interests []string `json:"interests"`
	Skills    string   `json:"skills"`
	Tech      string   `json:"tech"`
}
type teamView struct {
	ID int64 `json:"id"`
	teamBody
	Points int `json:"points"`
}

func viewTeam(t store.Team) teamView {
	return teamView{t.ID, teamBody{t.Name, t.Interests, t.Skills, t.Tech}, t.Points}
}

type taskBody struct {
	Company           string     `json:"company"`
	Title             string     `json:"title"`
	Industry          string     `json:"industry"`
	Category          string     `json:"category"`
	DraftText         string     `json:"draft_text"`
	QA                []store.QA `json:"qa"`
	Context           string     `json:"context"`
	Need              string     `json:"need"`
	Users             string     `json:"users"`
	Data              string     `json:"data"`
	Constraints       string     `json:"constraints"`
	ExpectedResult    string     `json:"expected_result"`
	SuccessCriteria   string     `json:"success_criteria"`
	Contact           string     `json:"contact"`
	InteractionFormat string     `json:"interaction_format"`
	Confirmed         []string   `json:"confirmed"`
	Reward            string     `json:"reward"`
	RewardType        string     `json:"reward_type"`
}

func (b taskBody) task() store.Task {
	return store.Task{Company: b.Company, Title: b.Title, Industry: b.Industry, Category: b.Category, DraftText: b.DraftText, QA: b.QA,
		Context: b.Context, Need: b.Need, Users: b.Users, Data: b.Data, Constraints: b.Constraints, ExpectedResult: b.ExpectedResult,
		SuccessCriteria: b.SuccessCriteria, Contact: b.Contact, InteractionFormat: b.InteractionFormat, Confirmed: b.Confirmed, Reward: b.Reward, RewardType: b.RewardType}
}

type taskView struct {
	ID int64 `json:"id"`
	taskBody
	Status string        `json:"status"`
	Rating rating.Result `json:"rating"`
}

func viewTask(t store.Task) taskView {
	return taskView{t.ID, taskBody{t.Company, t.Title, t.Industry, t.Category, t.DraftText, t.QA, t.Context, t.Need, t.Users, t.Data, t.Constraints, t.ExpectedResult, t.SuccessCriteria, t.Contact, t.InteractionFormat, t.Confirmed, t.Reward, t.RewardType}, t.Status, rating.Score(t.Card())}
}

func decodeFrontend(w http.ResponseWriter, r *http.Request, target any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "Отправьте данные в формате application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		writeDecodeError(w, err)
		return false
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		if err != nil {
			writeDecodeError(w, err)
		} else {
			writeError(w, http.StatusBadRequest, "После JSON обнаружены лишние данные")
		}
		return false
	}
	return true
}
func frontendError(w http.ResponseWriter, err error) {
	var validation *store.ValidationError
	switch {
	case errors.As(err, &validation):
		writeError(w, http.StatusBadRequest, validation.Error())
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "Запись не найдена")
	default:
		log.Printf("frontend API: %v", err)
		writeError(w, http.StatusInternalServerError, "Не удалось выполнить операцию с базой данных")
	}
}
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "Некорректный ID")
		return 0, false
	}
	return id, true
}
func (h frontendHandler) listTeams(w http.ResponseWriter, r *http.Request) {
	items, err := h.st.ListTeams(r.Context())
	if err != nil {
		frontendError(w, err)
		return
	}
	result := make([]teamView, 0, len(items))
	for _, t := range items {
		if session := requestSession(r); session != nil && session.Role == "student" && session.TeamID != 0 && session.TeamID != t.ID {
			continue
		}
		result = append(result, viewTeam(t))
	}
	writeJSON(w, http.StatusOK, result)
}
func (h frontendHandler) createTeam(w http.ResponseWriter, r *http.Request) {
	var b teamBody
	if !decodeFrontend(w, r, &b) {
		return
	}
	t := store.Team{Name: strings.TrimSpace(b.Name), Interests: b.Interests, Skills: strings.TrimSpace(b.Skills), Tech: strings.TrimSpace(b.Tech)}
	if err := h.st.CreateTeam(r.Context(), &t); err != nil {
		frontendError(w, err)
		return
	}
	if err := bindSessionTeam(h.st, r, t.ID); err != nil {
		frontendError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, viewTeam(t))
}
func (h frontendHandler) catalog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, err := h.st.ListCatalog(r.Context(), store.CatalogFilter{Category: q.Get("category"), Industry: q.Get("industry"), Level: q.Get("level")})
	if err != nil {
		frontendError(w, err)
		return
	}
	result := make([]taskView, 0, len(items))
	for _, t := range items {
		if session := requestSession(r); session != nil && session.Role == "business" && t.Company != session.Company {
			continue
		}
		result = append(result, viewTask(t))
	}
	writeJSON(w, http.StatusOK, result)
}
func (h frontendHandler) createTask(w http.ResponseWriter, r *http.Request) {
	var b taskBody
	if !decodeFrontend(w, r, &b) {
		return
	}
	if session := requestSession(r); session != nil && session.Role == "business" {
		if b.Company != "" && strings.TrimSpace(b.Company) != session.Company {
			writeError(w, 403, "Нельзя создать или изменить задачу от другой компании")
			return
		}
		b.Company = session.Company
	}
	t := b.task()
	if err := h.st.CreateTask(r.Context(), &t); err != nil {
		frontendError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, viewTask(t))
}
func (h frontendHandler) getTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	t, err := h.st.GetTask(r.Context(), id)
	if err != nil {
		frontendError(w, err)
		return
	}
	if session := requestSession(r); session != nil && session.Role == "student" && t.Status != store.StatusPublished {
		writeError(w, 404, "Задача не найдена")
		return
	}
	writeJSON(w, http.StatusOK, viewTask(t))
}
func (h frontendHandler) updateTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var b taskBody
	if !decodeFrontend(w, r, &b) {
		return
	}
	previous, err := h.st.GetTask(r.Context(), id)
	if err != nil {
		frontendError(w, err)
		return
	}
	if b.DraftText != previous.DraftText {
		writeError(w, http.StatusBadRequest, "Исходный черновик нельзя изменять после сохранения")
		return
	}
	if session := requestSession(r); session != nil && session.Role == "business" {
		if b.Company != "" && strings.TrimSpace(b.Company) != session.Company {
			writeError(w, 403, "Нельзя создать или изменить задачу от другой компании")
			return
		}
		b.Company = session.Company
	}
	t := b.task()
	t.ID = id
	if err := h.st.UpdateTask(r.Context(), &t); err != nil {
		frontendError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, viewTask(t))
}
func (h frontendHandler) publishTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var b struct{}
	if !decodeFrontend(w, r, &b) {
		return
	}
	if err := h.st.PublishTask(r.Context(), id); err != nil {
		frontendError(w, err)
		return
	}
	t, err := h.st.GetTask(r.Context(), id)
	if err != nil {
		frontendError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, viewTask(t))
}
