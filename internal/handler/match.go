package handler

import (
	"fmt"
	"log"
	"net/http"

	"github.com/leandrodaf/domino-placar/internal/db"
	"github.com/leandrodaf/domino-placar/internal/service"

	"github.com/google/uuid"
)

// CreateMatchHandler handles POST /match — creates a new match and redirects to lobby.
func CreateMatchHandler(database db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !CheckActionRateLimit(r) {
			http.Error(w, "too many requests, please wait", http.StatusTooManyRequests)
			return
		}

		scheme := "https"
		if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
			scheme = "http"
		}
		baseURL := fmt.Sprintf("%s://%s", scheme, r.Host)

		matchID := uuid.New().String()

		if err := database.CreateMatch(matchID, baseURL); err != nil {
			log.Printf("CreateMatch error: %v", err)
			http.Error(w, "failed to create match", http.StatusInternalServerError)
			return
		}

		// Set host cookie on the device that created the match
		SetHostCookie(w, matchID)

		http.Redirect(w, r, "/match/"+matchID+"/lobby", http.StatusSeeOther)
	}
}

// StartMatchHandler handles POST /match/{id}/start — starts the match.
func StartMatchHandler(database db.Store, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")

		if !IsHost(r, matchID) {
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}
		if !CheckActionRateLimit(r) {
			http.Error(w, "too many requests, please wait", http.StatusTooManyRequests)
			return
		}
		if err := r.ParseForm(); err == nil {
			if !ValidateCSRFToken(r.FormValue("_csrf"), matchID) {
				http.Error(w, "invalid security token", http.StatusForbidden)
				return
			}
		}

		players, err := database.GetPlayers(matchID)
		if err != nil || len(players) < 2 {
			http.Error(w, "need at least 2 players to start", http.StatusBadRequest)
			return
		}

		if err := database.UpdateMatchStatus(matchID, "active"); err != nil {
			log.Printf("UpdateMatchStatus error: %v", err)
			http.Error(w, "failed to start match", http.StatusInternalServerError)
			return
		}

		hub.Broadcast(matchID, "match_started")
		http.Redirect(w, r, "/match/"+matchID+"/ranking", http.StatusSeeOther)
	}
}

// QRCodeHandler serves the QR code image for a match join URL.
func QRCodeHandler(database db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		match, err := database.GetMatch(matchID)
		if err != nil {
			http.Error(w, "match not found", http.StatusNotFound)
			return
		}

		joinURL := fmt.Sprintf("%s/match/%s/join", match.BaseURL, matchID)
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
