package handler

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"regexp"
	"strings"

	"github.com/leandrodaf/domino-placar/internal/db"
	"github.com/leandrodaf/domino-placar/internal/models"
	"github.com/leandrodaf/domino-placar/internal/service"

	"github.com/google/uuid"
)

// generateInviteCode creates a random 6-character alphanumeric uppercase code.
func generateInviteCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no ambiguous chars (0,O,1,I)
	code := make([]byte, 6)
	for i := range code {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		code[i] = chars[n.Int64()]
	}
	return string(code)
}

var validName = regexp.MustCompile(`^[^\x00-\x1f<>&"'/\\]{1,50}$`)

// CreateTurmaPageHandler serves GET /turma/new — displays the create turma form.
func CreateTurmaPageHandler(tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl.Render(w, r, "turma-create.html", nil)
	}
}

// CreateTurmaHandler handles POST /turma — creates a new turma.
func CreateTurmaHandler(database db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !CheckActionRateLimit(r) {
			http.Error(w, "too many requests, please wait", http.StatusTooManyRequests)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		name := strings.TrimSpace(r.FormValue("name"))
		description := strings.TrimSpace(r.FormValue("description"))
		uniqueID := strings.TrimSpace(r.FormValue("unique_id"))
		playerName := strings.TrimSpace(r.FormValue("player_name"))
		isPrivate := r.FormValue("is_private") == "on"

		if name == "" || !validName.MatchString(name) {
			http.Error(w, "invalid turma name", http.StatusBadRequest)
			return
		}
		if uniqueID == "" || !validName.MatchString(uniqueID) {
			http.Error(w, "invalid unique identifier", http.StatusBadRequest)
			return
		}
		if playerName == "" || !validName.MatchString(playerName) {
			http.Error(w, "invalid player name", http.StatusBadRequest)
			return
		}
		if len(description) > 200 {
			description = description[:200]
		}

		scheme := "https"
		if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
			scheme = "http"
		}
		baseURL := fmt.Sprintf("%s://%s", scheme, r.Host)

		turmaID := uuid.New().String()
		inviteCode := generateInviteCode()

		turma := &models.Turma{
			ID:                turmaID,
			Name:              name,
			Description:       description,
			InviteCode:        inviteCode,
			IsPrivate:         isPrivate,
			CreatedByUniqueID: uniqueID,
			BaseURL:           baseURL,
		}

		if err := database.CreateTurma(turma); err != nil {
			log.Printf("CreateTurma error: %v", err)
			http.Error(w, "failed to create turma", http.StatusInternalServerError)
			return
		}

		// Add creator as admin member
		member := &models.TurmaMember{
			ID:               uuid.New().String(),
			TurmaID:          turmaID,
			UniqueIdentifier: uniqueID,
			Name:             playerName,
			Role:             "admin",
		}
		if err := database.AddTurmaMember(member); err != nil {
			log.Printf("AddTurmaMember error: %v", err)
		}

		// Set host cookie for the turma
		SetHostCookie(w, turmaID)

		http.Redirect(w, r, "/turma/"+turmaID, http.StatusSeeOther)
	}
}

// TurmaDashboardHandler serves GET /turma/{id} — the turma dashboard.
func TurmaDashboardHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		turmaID := r.PathValue("id")

		turma, err := database.GetTurma(turmaID)
		if err != nil {
			http.Error(w, "turma not found", http.StatusNotFound)
			return
		}

		members, _ := database.GetTurmaMembers(turmaID)
		matches, _ := database.GetTurmaMatches(turmaID)
		ranking, _ := database.GetTurmaRanking(turmaID)

		// Limit ranking to top 5 for dashboard
		if len(ranking) > 5 {
			ranking = ranking[:5]
		}
		// Limit matches to recent 5
		if len(matches) > 5 {
			matches = matches[:5]
		}

		tmpl.Render(w, r, "turma-dashboard.html", map[string]any{
			"Turma":     turma,
			"Members":   members,
			"Matches":   matches,
			"Ranking":   ranking,
			"IsHost":    IsHost(r, turmaID),
			"CSRFToken": GenerateCSRFToken(turmaID),
		})
	}
}

// JoinTurmaPageHandler serves GET /turma/{id}/join — the join form.
func JoinTurmaPageHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		turmaID := r.PathValue("id")

		turma, err := database.GetTurma(turmaID)
		if err != nil {
			http.Error(w, "turma not found", http.StatusNotFound)
			return
		}

		// If turma is private and no code provided in query, show code-required message
		code := r.URL.Query().Get("code")
		needsCode := turma.IsPrivate && code != turma.InviteCode
		errorCode := r.URL.Query().Get("error") == "invalid_code"

		tmpl.Render(w, r, "turma-join.html", map[string]any{
			"Turma":     turma,
			"NeedsCode": needsCode,
			"ErrorCode": errorCode,
		})
	}
}

// JoinTurmaHandler handles POST /turma/{id}/join — registers a new member.
func JoinTurmaHandler(database db.Store, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		turmaID := r.PathValue("id")

		if !CheckActionRateLimit(r) {
			http.Error(w, "too many requests, please wait", http.StatusTooManyRequests)
			return
		}

		turma, err := database.GetTurma(turmaID)
		if err != nil {
			http.Error(w, "turma not found", http.StatusNotFound)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		name := strings.TrimSpace(r.FormValue("name"))
		uniqueID := strings.TrimSpace(r.FormValue("unique_id"))
		code := strings.ToUpper(strings.TrimSpace(r.FormValue("invite_code")))

		if name == "" || !validName.MatchString(name) || uniqueID == "" || !validName.MatchString(uniqueID) {
			http.Error(w, "invalid input", http.StatusBadRequest)
			return
		}

		// Check private turma code
		if turma.IsPrivate && code != turma.InviteCode {
			http.Redirect(w, r, "/turma/"+turmaID+"/join?error=invalid_code", http.StatusSeeOther)
			return
		}

		// Check if already a member
		if ok, _ := database.IsTurmaMember(turmaID, uniqueID); ok {
			http.Redirect(w, r, "/turma/"+turmaID, http.StatusSeeOther)
			return
		}

		member := &models.TurmaMember{
			ID:               uuid.New().String(),
			TurmaID:          turmaID,
			UniqueIdentifier: uniqueID,
			Name:             name,
			Role:             "member",
		}

		if err := database.AddTurmaMember(member); err != nil {
			log.Printf("AddTurmaMember error: %v", err)
			http.Error(w, "failed to join turma", http.StatusInternalServerError)
			return
		}

		hub.Broadcast(turmaID, "member_joined")
		http.Redirect(w, r, "/turma/"+turmaID, http.StatusSeeOther)
	}
}

// JoinByCodeHandler serves GET /turma/join?code={code} — finds turma by invite code.
func JoinByCodeHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			tmpl.Render(w, r, "turma-join-code.html", nil)
			return
		}

		code = strings.ToUpper(code)
		turma, err := database.GetTurmaByInviteCode(code)
		if err != nil {
			tmpl.Render(w, r, "turma-join-code.html", map[string]any{
				"NotFound": true,
				"Code":     code,
			})
			return
		}

		http.Redirect(w, r, "/turma/"+turma.ID+"/join?code="+turma.InviteCode, http.StatusSeeOther)
	}
}

// TurmaRankingHandler serves GET /turma/{id}/ranking — full ranking page.
func TurmaRankingHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		turmaID := r.PathValue("id")

		turma, err := database.GetTurma(turmaID)
		if err != nil {
			http.Error(w, "turma not found", http.StatusNotFound)
			return
		}

		ranking, _ := database.GetTurmaRanking(turmaID)

		tmpl.Render(w, r, "turma-ranking.html", map[string]any{
			"Turma":   turma,
			"Ranking": ranking,
		})
	}
}

// CreateMatchInTurmaHandler handles POST /turma/{id}/match — creates a match in the turma.
func CreateMatchInTurmaHandler(database db.Store, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		turmaID := r.PathValue("id")

		if !CheckActionRateLimit(r) {
			http.Error(w, "too many requests, please wait", http.StatusTooManyRequests)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		if !ValidateCSRFToken(r.FormValue("_csrf"), turmaID) {
			http.Error(w, "invalid security token", http.StatusForbidden)
			return
		}

		// Check membership — require unique_id from form or cookie context
		uniqueID := strings.TrimSpace(r.FormValue("unique_id"))
		if uniqueID == "" {
			http.Error(w, "missing identifier", http.StatusBadRequest)
			return
		}
		if isMember, _ := database.IsTurmaMember(turmaID, uniqueID); !isMember {
			http.Error(w, "access denied — not a turma member", http.StatusForbidden)
			return
		}

		if _, err := database.GetTurma(turmaID); err != nil {
			http.Error(w, "turma not found", http.StatusNotFound)
			return
		}

		scheme := "https"
		if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
			scheme = "http"
		}
		baseURL := fmt.Sprintf("%s://%s", scheme, r.Host)

		matchID := uuid.New().String()
		if err := database.CreateMatchInTurma(matchID, baseURL, turmaID); err != nil {
			log.Printf("CreateMatchInTurma error: %v", err)
			http.Error(w, "failed to create match", http.StatusInternalServerError)
			return
		}

		SetHostCookie(w, matchID)
		hub.Broadcast(turmaID, "match_created")
		http.Redirect(w, r, "/match/"+matchID+"/lobby", http.StatusSeeOther)
	}
}

// TurmaQRCodeHandler serves the QR code image for a turma join URL.
func TurmaQRCodeHandler(database db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		turmaID := r.PathValue("id")
		turma, err := database.GetTurma(turmaID)
		if err != nil {
			http.Error(w, "turma not found", http.StatusNotFound)
			return
		}

		joinURL := fmt.Sprintf("%s/turma/%s/join", turma.BaseURL, turmaID)
		png, err := service.GenerateQRCode(joinURL)
		if err != nil {
			log.Printf("GenerateQRCode error: %v", err)
			http.Error(w, "failed to generate QR code", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "image/png")
		w.Write(png)
	}
}

// RemoveTurmaMemberHandler handles POST /turma/{id}/remove-member/{uid} — admin removes a member.
func RemoveTurmaMemberHandler(database db.Store, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		turmaID := r.PathValue("id")
		targetUID := r.PathValue("uid")

		if !IsHost(r, turmaID) {
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}
		if !CheckActionRateLimit(r) {
			http.Error(w, "too many requests, please wait", http.StatusTooManyRequests)
			return
		}
		if err := r.ParseForm(); err == nil {
			if !ValidateCSRFToken(r.FormValue("_csrf"), turmaID) {
				http.Error(w, "invalid security token", http.StatusForbidden)
				return
			}
		}

		turma, err := database.GetTurma(turmaID)
		if err != nil {
			http.Error(w, "turma not found", http.StatusNotFound)
			return
		}

		// Cannot remove the creator/admin
		if targetUID == turma.CreatedByUniqueID {
			http.Error(w, "cannot remove the turma creator", http.StatusBadRequest)
			return
		}

		if err := database.RemoveTurmaMember(turmaID, targetUID); err != nil {
			log.Printf("RemoveTurmaMember error: %v", err)
			http.Error(w, "failed to remove member", http.StatusInternalServerError)
			return
		}

		hub.Broadcast(turmaID, "member_removed")
		http.Redirect(w, r, "/turma/"+turmaID, http.StatusSeeOther)
	}
}

// TurmasByMemberHandler serves GET /turma/my?unique_id={uid} — returns turmas for a user (JSON).
func TurmasByMemberHandler(database db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uniqueID := strings.TrimSpace(r.URL.Query().Get("unique_id"))
		if uniqueID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
			return
		}

		turmas, err := database.GetTurmasByMember(uniqueID)
		if err != nil || len(turmas) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
			return
		}

		type turmaJSON struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			InviteCode string `json:"invite_code"`
		}
		result := make([]turmaJSON, len(turmas))
		for i, t := range turmas {
			result[i] = turmaJSON{ID: t.ID, Name: t.Name, InviteCode: t.InviteCode}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}
