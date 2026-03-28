package handler

import (
	"log"
	"net/http"

	"github.com/leandrodaf/domino-placar/internal/db"
	"github.com/leandrodaf/domino-placar/internal/models"
)

// TournamentLobbyHandler handles GET /tournament/{id}/lobby — shows the host lobby.
func TournamentLobbyHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tournamentID := r.PathValue("id")

		tournament, err := database.GetTournament(tournamentID)
		if err != nil {
			http.Error(w, "tournament not found", http.StatusNotFound)
			return
		}

		players, err := database.GetTournamentPlayers(tournamentID)
		if err != nil {
			players = nil
		}

		groups := allocateGroups(len(players))

		data := map[string]any{
			"Tournament": tournament,
			"Players":    players,
			"Count":      len(players),
			"MinPlayers": 2,
			"Groups":     groups,
		}
		tmpl.Render(w, r, "tournament-lobby.html", data)
	}
}

// TournamentJoinPageHandler handles GET /tournament/{id}/join — shows the player join form.
func TournamentJoinPageHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tournamentID := r.PathValue("id")

		tournament, err := database.GetTournament(tournamentID)
		if err != nil {
			http.Error(w, "tournament not found", http.StatusNotFound)
			return
		}

		var errMsg string
		switch r.URL.Query().Get("error") {
		case "full":
			errMsg = "This tournament is full (max 60 players)."
		case "started":
			errMsg = "This tournament has already started and is not accepting new players."
		case "missing":
			errMsg = "Please fill in all fields."
		}

		data := map[string]any{
			"Tournament": tournament,
			"Error":      errMsg,
		}
		tmpl.Render(w, r, "tournament-join.html", data)
	}
}

// TournamentWaitingHandler handles GET /tournament/{id}/waiting — shows waiting room for players.
func TournamentWaitingHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tournamentID := r.PathValue("id")

		tournament, err := database.GetTournament(tournamentID)
		if err != nil {
			http.Error(w, "tournament not found", http.StatusNotFound)
			return
		}

		if tournament.Status == "active" {
			http.Redirect(w, r, "/tournament/"+tournamentID+"/tables", http.StatusSeeOther)
			return
		}

		playerID := r.URL.Query().Get("player_id")
		var player *models.TournamentPlayer
		if playerID != "" {
			players, _ := database.GetTournamentPlayers(tournamentID)
			for i := range players {
				if players[i].ID == playerID {
					player = &players[i]
					break
				}
			}
		}

		data := map[string]any{
			"Tournament": tournament,
			"Player":     player,
		}
		tmpl.Render(w, r, "tournament-waiting.html", data)
	}
}

// TableInfo holds the data for one table in the tournament tables view.
type TableInfo struct {
	TableNumber int
	MatchID     string
	Match       *models.Match
	Players     []models.Player
}

// TournamentTablesHandler handles GET /tournament/{id}/tables — shows all tables.
func TournamentTablesHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tournamentID := r.PathValue("id")

		tournament, err := database.GetTournament(tournamentID)
		if err != nil {
			http.Error(w, "tournament not found", http.StatusNotFound)
			return
		}

		tournamentMatches, err := database.GetTournamentMatches(tournamentID)
		if err != nil {
			log.Printf("GetTournamentMatches error: %v", err)
			tournamentMatches = nil
		}

		var tables []TableInfo
		for _, tm := range tournamentMatches {
			match, err := database.GetMatch(tm.MatchID)
			if err != nil {
				log.Printf("GetMatch error for %s: %v", tm.MatchID, err)
				continue
			}
			players, err := database.GetPlayers(tm.MatchID)
			if err != nil {
				players = nil
			}
			tables = append(tables, TableInfo{
				TableNumber: tm.TableNumber,
				MatchID:     tm.MatchID,
				Match:       match,
				Players:     players,
			})
		}

		data := map[string]any{
			"Tournament": tournament,
			"Tables":     tables,
		}
		tmpl.Render(w, r, "tournament-tables.html", data)
	}
}

// TournamentRankingHandler handles GET /tournament/{id}/ranking — shows overall tournament ranking.
func TournamentRankingHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tournamentID := r.PathValue("id")

		tournament, err := database.GetTournament(tournamentID)
		if err != nil {
			http.Error(w, "tournament not found", http.StatusNotFound)
			return
		}

		ranking, err := database.GetTournamentRanking(tournamentID)
		if err != nil {
			log.Printf("GetTournamentRanking error: %v", err)
			ranking = nil
		}

		data := map[string]any{
			"Tournament": tournament,
			"Ranking":    ranking,
		}
		tmpl.Render(w, r, "tournament-ranking.html", data)
	}
}
