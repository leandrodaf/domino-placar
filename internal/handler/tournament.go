package handler

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"

	"github.com/leandrodaf/domino-placar/internal/db"
	"github.com/leandrodaf/domino-placar/internal/service"

	"github.com/google/uuid"
)

func allocateGroups(n int) []int {
	if n < 2 {
		return nil
	}
	numFours := n / 4
	remainder := n % 4
	groups := make([]int, numFours)
	for i := range groups {
		groups[i] = 4
	}
	switch remainder {
	case 0:
	case 1:
		if numFours >= 1 {
			groups[len(groups)-1] = 3
			groups = append(groups, 2)
		} else {
			return nil
		}
	case 2:
		groups = append(groups, 2)
	case 3:
		groups = append(groups, 3)
	}
	return groups
}

// CreateTournamentHandler handles POST /tournament — creates a new tournament.
func CreateTournamentHandler(database db.Store) http.HandlerFunc {
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

		tournamentID := uuid.New().String()

		if err := database.CreateTournament(tournamentID, "Tournament", baseURL); err != nil {
			log.Printf("CreateTournament error: %v", err)
			http.Error(w, "failed to create tournament", http.StatusInternalServerError)
			return
		}

		// Store the host cookie for the tournament
		SetHostCookie(w, tournamentID)

		http.Redirect(w, r, "/tournament/"+tournamentID+"/lobby", http.StatusSeeOther)
	}
}

// JoinTournamentHandler handles POST /tournament/{id}/join — registers a player for the tournament.
func JoinTournamentHandler(database db.Store, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tournamentID := r.PathValue("id")

		if !CheckActionRateLimit(r) {
			http.Error(w, "too many requests, please wait", http.StatusTooManyRequests)
			return
		}

		tournament, err := database.GetTournament(tournamentID)
		if err != nil {
			http.Error(w, "tournament not found", http.StatusNotFound)
			return
		}

		if tournament.Status != "registration" {
			http.Redirect(w, r, "/tournament/"+tournamentID+"/join?error=started", http.StatusSeeOther)
			return
		}

		count, err := database.CountTournamentPlayers(tournamentID)
		if err != nil {
			log.Printf("CountTournamentPlayers error: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if count >= 60 {
			http.Redirect(w, r, "/tournament/"+tournamentID+"/join?error=full", http.StatusSeeOther)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}

		name := SanitizeInput(r.FormValue("name"), 50)
		uniqueID := SanitizeInput(r.FormValue("unique_id"), 50)

		if name == "" || uniqueID == "" {
			http.Redirect(w, r, "/tournament/"+tournamentID+"/join?error=missing", http.StatusSeeOther)
			return
		}

		existing, _ := database.GetTournamentPlayerByUniqueID(tournamentID, uniqueID)
		if existing != nil {
			http.Redirect(w, r, "/tournament/"+tournamentID+"/waiting?player_id="+existing.ID, http.StatusSeeOther)
			return
		}

		playerID := uuid.New().String()
		if err := database.CreateTournamentPlayer(playerID, tournamentID, name, uniqueID); err != nil {
			log.Printf("CreateTournamentPlayer error: %v", err)
			http.Error(w, "failed to register player", http.StatusInternalServerError)
			return
		}

		hub.Broadcast(tournamentID, "player_joined")

		http.Redirect(w, r, "/tournament/"+tournamentID+"/waiting?player_id="+playerID, http.StatusSeeOther)
	}
}

// StartTournamentHandler handles POST /tournament/{id}/start — allocates tables and starts the tournament.
func StartTournamentHandler(database db.Store, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tournamentID := r.PathValue("id")

		if !IsHost(r, tournamentID) {
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}
		if !CheckActionRateLimit(r) {
			http.Error(w, "too many requests, please wait", http.StatusTooManyRequests)
			return
		}

		tournament, err := database.GetTournament(tournamentID)
		if err != nil {
			http.Error(w, "tournament not found", http.StatusNotFound)
			return
		}

		players, err := database.GetTournamentPlayers(tournamentID)
		if err != nil || len(players) < 2 {
			http.Error(w, "need at least 2 players to start", http.StatusBadRequest)
			return
		}

		groups := allocateGroups(len(players))
		if groups == nil {
			http.Error(w, "cannot allocate groups", http.StatusBadRequest)
			return
		}

		rand.Shuffle(len(players), func(i, j int) {
			players[i], players[j] = players[j], players[i]
		})

		idx := 0
		for tableNum, groupSize := range groups {
			tableNumber := tableNum + 1

			matchID := uuid.New().String()
			if err := database.CreateMatch(matchID, tournament.BaseURL); err != nil {
				log.Printf("CreateMatch error (table %d): %v", tableNumber, err)
				http.Error(w, "failed to create match", http.StatusInternalServerError)
				return
			}

			if err := database.UpdateMatchStatus(matchID, "waiting"); err != nil {
				log.Printf("UpdateMatchStatus error: %v", err)
			}

			if err := database.CreateTournamentMatch(tournamentID, matchID, tableNumber); err != nil {
				log.Printf("CreateTournamentMatch error: %v", err)
				http.Error(w, "failed to link tournament match", http.StatusInternalServerError)
				return
			}

			for i := 0; i < groupSize && idx < len(players); i++ {
				tp := players[idx]
				idx++

				playerID := uuid.New().String()
				if err := database.CreatePlayer(playerID, matchID, tp.Name, tp.UniqueIdentifier); err != nil {
					log.Printf("CreatePlayer error: %v", err)
					http.Error(w, "failed to create match player", http.StatusInternalServerError)
					return
				}

				if err := database.AssignTournamentPlayer(tp.ID, tableNumber, matchID); err != nil {
					log.Printf("AssignTournamentPlayer error: %v", err)
				}
			}

			if err := database.UpdateMatchStatus(matchID, "active"); err != nil {
				log.Printf("UpdateMatchStatus to active error: %v", err)
			}
		}

		if err := database.UpdateTournamentStatus(tournamentID, "active"); err != nil {
			log.Printf("UpdateTournamentStatus error: %v", err)
			http.Error(w, "failed to update tournament status", http.StatusInternalServerError)
			return
		}

		hub.Broadcast(tournamentID, "tournament_started")
		http.Redirect(w, r, "/tournament/"+tournamentID+"/tables", http.StatusSeeOther)
	}
}

// TournamentQRCodeHandler handles GET /tournament/{id}/qrcode.
func TournamentQRCodeHandler(database db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tournamentID := r.PathValue("id")

		tournament, err := database.GetTournament(tournamentID)
		if err != nil {
			http.Error(w, "tournament not found", http.StatusNotFound)
			return
		}

		joinURL := fmt.Sprintf("%s/tournament/%s/join", tournament.BaseURL, tournamentID)
		png, err := service.GenerateQRCode(joinURL)
		if err != nil {
			log.Printf("GenerateQRCode error: %v", err)
			http.Error(w, "failed to generate QR code", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	}
}

// TournamentPlayersPartialHandler handles GET /tournament/{id}/players-partial.
func TournamentPlayersPartialHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tournamentID := r.PathValue("id")

		players, err := database.GetTournamentPlayers(tournamentID)
		if err != nil {
			players = nil
		}

		groups := allocateGroups(len(players))

		tmpl.RenderPartial(w, r, "tournament-players-partial.html", map[string]any{
			"Players":      players,
			"Count":        len(players),
			"TournamentID": tournamentID,
			"Groups":       groups,
		})
	}
}
