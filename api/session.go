package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BAITC-Hacks/hack-7c4211c3-kilc/store"
)

type browserSession struct {
	Company string `json:"company"`
	Role    string `json:"role"`
	TeamID  int64  `json:"team_id"`
	token   string
}
type sessionContextKey struct{}

func requestSession(r *http.Request) *browserSession {
	s, _ := r.Context().Value(sessionContextKey{}).(*browserSession)
	return s
}
func sessionHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// SessionAccess binds one role and one team to an opaque browser cookie.
// This is a persistent browser identity, not password-based user authentication.
func SessionAccess(st *store.Store, next http.Handler) http.Handler {
	var selection sync.Mutex
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if r.Method == "POST" && (p == "/api/session" || p == "/api/session/team" || p == "/api/session/company" || p == "/api/teams") {
			selection.Lock()
			defer selection.Unlock()
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host && origin != "https://"+r.Host {
				writeError(w, 403, "Запрос с другого сайта запрещён")
				return
			}
		}
		s := &browserSession{}
		if cookie, err := r.Cookie("tasklab-session"); err == nil {
			s.token = sessionHash(cookie.Value)
			err := st.DB.QueryRowContext(r.Context(), `SELECT role, COALESCE(team_id,0), company FROM browser_sessions WHERE token=? AND expires_at>?`, s.token, time.Now().Unix()).Scan(&s.Role, &s.TeamID, &s.Company)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				frontendError(w, err)
				return
			}
		}
		r = r.WithContext(context.WithValue(r.Context(), sessionContextKey{}, s))
		if p == "/api/session" {
			w.Header().Set("Cache-Control", "no-store")
			if r.Method == "GET" {
				writeJSON(w, 200, s)
				return
			}
			if r.Method != "POST" {
				writeError(w, 405, "Используйте GET или POST")
				return
			}
			var body struct {
				Role string `json:"role"`
			}
			if !decodeFrontend(w, r, &body) {
				return
			}
			if body.Role != "student" && body.Role != "business" {
				writeError(w, 400, "Выберите роль студента или бизнеса")
				return
			}
			if s.Role != "" {
				if s.Role != body.Role {
					writeError(w, 409, "Роль уже выбрана. Сменить её в этой сессии нельзя")
					return
				}
				writeJSON(w, 200, s)
				return
			}
			var token [32]byte
			if _, err := rand.Read(token[:]); err != nil {
				frontendError(w, err)
				return
			}
			raw := hex.EncodeToString(token[:])
			s.token = sessionHash(raw)
			s.Role = body.Role
			expires := time.Now().Add(365 * 24 * time.Hour)
			if _, err := st.DB.ExecContext(r.Context(), `INSERT INTO browser_sessions(token,role,expires_at) VALUES(?,?,?)`, s.token, s.Role, expires.Unix()); err != nil {
				frontendError(w, err)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "tasklab-session", Value: raw, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil, MaxAge: 365 * 24 * 60 * 60, Expires: expires})
			writeJSON(w, 201, s)
			return
		}
		// Assets and the initial role selector must load before a session exists.
		if p == "/health" || p == "/api/categories" || p == "/api/reward-types" || strings.HasPrefix(p, "/static/") && !strings.HasSuffix(p, ".html") && !strings.HasSuffix(p, "/") {
			next.ServeHTTP(w, r)
			return
		}
		login := p == "/static/task-builder/login.html" || p == "/static/task-builder/register.html"
		if login && s.Role == "" {
			next.ServeHTTP(w, r)
			return
		}
		if s.Role == "" {
			if strings.HasPrefix(p, "/api/") {
				writeError(w, 401, "Сначала выберите роль")
				return
			}
			http.Redirect(w, r, "/static/task-builder/login.html", http.StatusSeeOther)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		home := "/static/task-builder/business.html"
		if s.Role == "student" {
			home = "/static/task-builder/teams.html"
		}
		if login {
			http.Redirect(w, r, home, http.StatusSeeOther)
			return
		}
		businessPage := p == "/static/task-builder/index.html" || p == "/static/task-builder/" || p == "/static/task-builder/business.html" || p == "/static/task-builder/team-catalog.html" || p == "/static/task-builder/company.html"
		if businessPage && s.Role != "business" || p == "/static/task-builder/teams.html" && s.Role != "student" {
			http.Redirect(w, r, home, http.StatusSeeOther)
			return
		}
		if p == "/api/session/team" {
			if r.Method != "POST" {
				writeError(w, 405, "Используйте POST")
				return
			}
			if s.Role != "student" {
				writeError(w, 403, "Команда доступна только студенту")
				return
			}
			var body struct {
				TeamID int64 `json:"team_id"`
			}
			if !decodeFrontend(w, r, &body) {
				return
			}
			if _, err := st.GetTeam(r.Context(), body.TeamID); err != nil {
				frontendError(w, err)
				return
			}
			if s.TeamID != 0 && s.TeamID != body.TeamID {
				writeError(w, 409, "Вы уже состоите в команде. Смена команды недоступна")
				return
			}
			if err := bindSessionTeam(st, r, body.TeamID); err != nil {
				frontendError(w, err)
				return
			}
			s.TeamID = body.TeamID
			writeJSON(w, 200, s)
			return
		}
		if p == "/api/session/company" {
			if r.Method != "POST" {
				writeError(w, 405, "Используйте POST")
				return
			}
			if s.Role != "business" {
				writeError(w, 403, "Компания доступна только бизнесу")
				return
			}
			var body struct {
				Company string `json:"company"`
			}
			if !decodeFrontend(w, r, &body) {
				return
			}
			body.Company = strings.TrimSpace(body.Company)
			if body.Company == "" || len([]rune(body.Company)) > 250 {
				writeError(w, 400, "Укажите название компании: от 1 до 250 символов")
				return
			}
			if s.Company != "" && s.Company != body.Company {
				writeError(w, 409, "Компания уже закреплена за этой сессией")
				return
			}
			if _, err := st.DB.ExecContext(r.Context(), `UPDATE browser_sessions SET company=? WHERE token=? AND role='business' AND (company='' OR company=?)`, body.Company, s.token, body.Company); err != nil {
				frontendError(w, err)
				return
			}
			s.Company = body.Company
			writeJSON(w, 200, s)
			return
		}
		if s.Role == "business" {
			if s.Company == "" && p != "/static/task-builder/company.html" && p != "/api/business/companies" {
				if strings.HasPrefix(p, "/api/") {
					writeError(w, 403, "Сначала укажите вашу компанию")
					return
				}
				http.Redirect(w, r, "/static/task-builder/company.html", http.StatusSeeOther)
				return
			}
			if p == "/" || p == "/static/task-builder/company.html" && s.Company != "" {
				http.Redirect(w, r, home, http.StatusSeeOther)
				return
			}
			var taskID int64
			if strings.HasPrefix(p, "/api/tasks/") {
				taskID, _ = strconv.ParseInt(strings.Split(strings.TrimPrefix(p, "/api/tasks/"), "/")[0], 10, 64)
			} else if strings.HasPrefix(p, "/api/proposals/") {
				proposalID, _ := strconv.ParseInt(strings.Split(strings.TrimPrefix(p, "/api/proposals/"), "/")[0], 10, 64)
				err := st.DB.QueryRowContext(r.Context(), `SELECT task_id FROM proposals WHERE id=?`, proposalID).Scan(&taskID)
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, 404, "Предложение не найдено")
					return
				}
				if err != nil {
					frontendError(w, err)
					return
				}
			}
			if taskID > 0 {
				task, err := st.GetTask(r.Context(), taskID)
				if err != nil {
					frontendError(w, err)
					return
				}
				if task.Company != s.Company {
					writeError(w, 403, "Эта задача принадлежит другой компании")
					return
				}
			}
		}
		businessAPI := strings.HasPrefix(p, "/api/business/") || strings.HasPrefix(p, "/api/proposals/") || strings.HasSuffix(p, "/proposals/full") || p == "/api/rating"
		taskWrite := strings.HasPrefix(p, "/api/tasks") && r.Method != "GET" && r.Method != "HEAD" && !strings.HasSuffix(p, "/proposals")
		if (businessAPI || taskWrite) && s.Role != "business" {
			writeError(w, 403, "Действие доступно только бизнесу")
			return
		}
		if p == "/api/teams" && r.Method == "POST" && (s.Role != "student" || s.TeamID != 0) {
			writeError(w, 403, "Создание команды недоступно: роль или команда уже определена")
			return
		}
		if strings.HasSuffix(p, "/recommendations") && strings.HasPrefix(p, "/api/teams/") {
			id, _ := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(p, "/api/teams/"), "/recommendations"), 10, 64)
			if s.Role != "student" || s.TeamID == 0 || id != s.TeamID {
				writeError(w, 403, "Доступны рекомендации только вашей команды")
				return
			}
		}
		if strings.HasSuffix(p, "/proposals") && r.Method == "POST" {
			if s.Role != "student" || s.TeamID == 0 {
				writeError(w, 403, "Сначала вступите в команду")
				return
			}
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64*1024))
			if err != nil {
				writeError(w, 400, "Не удалось прочитать предложение")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			var proposal proposalBody
			if json.Unmarshal(body, &proposal) != nil {
				writeError(w, 400, "Некорректное предложение")
				return
			}
			if proposal.TeamID != s.TeamID {
				writeError(w, 403, "Отправить предложение можно только от своей команды")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func bindSessionTeam(st *store.Store, r *http.Request, id int64) error {
	s := requestSession(r)
	if s == nil {
		return nil
	}
	result, err := st.DB.ExecContext(r.Context(), `UPDATE browser_sessions SET team_id=? WHERE token=? AND role='student' AND (team_id IS NULL OR team_id=?)`, id, s.token, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return &store.ValidationError{Field: "team_id", Message: "Команда уже выбрана"}
	}
	return nil
}
