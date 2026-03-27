package handler

import (
	"log"
	"net/http"

	"github.com/leandrodaf/domino-placar/internal/db"
	"github.com/leandrodaf/domino-placar/internal/models"

	"github.com/google/uuid"
)

// CreateRoundHandler handles POST /match/{id}/round — creates a new round.
func CreateRoundHandler(database db.Store, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")

		if !IsHost(r, matchID) {
			http.Error(w, "acesso negado", http.StatusForbidden)
			return
		}
		if !CheckActionRateLimit(r) {
			http.Error(w, "muitas requisições, aguarde", http.StatusTooManyRequests)
			return
		}
		if err := r.ParseForm(); err != nil || !ValidateCSRFToken(r.FormValue("_csrf"), matchID) {
			http.Error(w, "token de segurança inválido", http.StatusForbidden)
			return
		}

		count, err := database.CountRounds(matchID)
		if err != nil {
			log.Printf("CountRounds error: %v", err)
			http.Error(w, "failed to count rounds", http.StatusInternalServerError)
			return
		}

		// Bloqueia se ainda há rodada ativa não finalizada
		if count > 0 {
			if prev, err := database.GetCurrentRound(matchID); err == nil && prev.Status == "active" {
				http.Redirect(w, r, "/match/"+matchID+"/round/"+prev.ID+"/round-scores?error=incomplete", http.StatusSeeOther)
				return
			}
		}

		roundID := uuid.New().String()
		if err := database.CreateRound(roundID, matchID, count+1); err != nil {
			log.Printf("CreateRound error: %v", err)
			http.Error(w, "failed to create round", http.StatusInternalServerError)
			return
		}

		// A partir da rodada 2, o iniciador é o jogador à esquerda do iniciador anterior
		if count >= 1 {
			if prev, err := database.GetLastFinishedRound(matchID); err == nil && prev.StarterPlayerID != "" {
				if players, err := database.GetPlayers(matchID); err == nil {
					var active []models.Player
					for _, p := range players {
						if p.Status == "active" {
							active = append(active, p)
						}
					}
					for i, p := range active {
						if p.ID == prev.StarterPlayerID {
							next := active[(i+1)%len(active)]
							if err2 := database.SetRoundStarter(roundID, next.ID); err2 != nil {
								log.Printf("SetRoundStarter auto error: %v", err2)
							}
							break
						}
					}
				}
			}
		}

		hub.Broadcast(matchID, "round_started:"+roundID)
		http.Redirect(w, r, "/match/"+matchID+"/round/"+roundID+"/game", http.StatusSeeOther)
	}
}

// SetRoundStarterHandler handles POST /match/{id}/round/{rid}/starter/{pid} — marks who played first.
func SetRoundStarterHandler(database db.Store, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		roundID := r.PathValue("rid")
		playerID := r.PathValue("pid")

		if !IsHost(r, matchID) {
			http.Error(w, "acesso negado", http.StatusForbidden)
			return
		}
		if !CheckActionRateLimit(r) {
			http.Error(w, "muitas requisições, aguarde", http.StatusTooManyRequests)
			return
		}
		if err := r.ParseForm(); err != nil || !ValidateCSRFToken(r.FormValue("_csrf"), matchID) {
			http.Error(w, "token de segurança inválido", http.StatusForbidden)
			return
		}

		// Valida que o jogador pertence à partida
		player, err := database.GetPlayer(playerID)
		if err != nil || player.MatchID != matchID {
			http.Error(w, "jogador não encontrado", http.StatusBadRequest)
			return
		}

		if err := database.SetRoundStarter(roundID, playerID); err != nil {
			log.Printf("SetRoundStarter error: %v", err)
			http.Error(w, "failed to set starter", http.StatusInternalServerError)
			return
		}

		hub.Broadcast(matchID, "round_started:"+roundID)
		http.Redirect(w, r, "/match/"+matchID+"/round/"+roundID+"/game", http.StatusSeeOther)
	}
}

// SetRoundWinnerHandler handles POST /match/{id}/round/{rid}/winner/{pid} — sets the round winner.
func SetRoundWinnerHandler(database db.Store, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		roundID := r.PathValue("rid")
		playerID := r.PathValue("pid")

		if !IsHost(r, matchID) {
			http.Error(w, "acesso negado", http.StatusForbidden)
			return
		}
		if !CheckActionRateLimit(r) {
			http.Error(w, "muitas requisições, aguarde", http.StatusTooManyRequests)
			return
		}
		if err := r.ParseForm(); err != nil || !ValidateCSRFToken(r.FormValue("_csrf"), matchID) {
			http.Error(w, "token de segurança inválido", http.StatusForbidden)
			return
		}

		// Valida que o jogador pertence à partida
		player, err := database.GetPlayer(playerID)
		if err != nil || player.MatchID != matchID {
			http.Error(w, "jogador não encontrado", http.StatusBadRequest)
			return
		}

		if err := database.SetRoundWinner(roundID, playerID); err != nil {
			log.Printf("SetRoundWinner error: %v", err)
			http.Error(w, "failed to set winner", http.StatusInternalServerError)
			return
		}

		hub.Broadcast(matchID, "round_winner_set:"+playerID)
		// Host vai direto para entrada de pontos em lote; jogadores atualizam via SSE
		http.Redirect(w, r, "/match/"+matchID+"/round/"+roundID+"/round-scores", http.StatusSeeOther)
	}
}

// findMatchWinner retorna o ID do vencedor: o único ativo restante, ou o de menor pontuação.
func findMatchWinner(players []models.Player) string {
	var active []models.Player
	for _, p := range players {
		if p.Status == "active" {
			active = append(active, p)
		}
	}
	if len(active) == 1 {
		return active[0].ID
	}
	// Todos estouraram — menor pontuação vence
	if len(active) == 0 && len(players) > 0 {
		best := players[0]
		for _, p := range players[1:] {
			if p.TotalScore < best.TotalScore {
				best = p
			}
		}
		return best.ID
	}
	return ""
}

// checkRoundComplete verifica se todos os jogadores ativos confirmaram e finaliza a rodada.
func checkRoundComplete(database db.Store, hub *SSEHub, matchID, roundID string) error {
	round, err := database.GetRound(roundID)
	if err != nil {
		return err
	}

	if round.Status != "active" {
		return nil
	}

	if round.WinnerPlayerID == "" {
		return nil
	}

	players, err := database.GetPlayers(matchID)
	if err != nil {
		return err
	}

	handImages, err := database.GetHandImages(roundID)
	if err != nil {
		return err
	}

	confirmed := map[string]*models.HandImage{}
	for i := range handImages {
		hi := &handImages[i]
		if hi.Confirmed == 1 {
			confirmed[hi.PlayerID] = hi
		}
	}

	activePlayers := 0
	confirmedNonWinners := 0
	for _, p := range players {
		if p.Status != "active" {
			continue
		}
		activePlayers++
		if p.ID == round.WinnerPlayerID {
			continue
		}
		if _, ok := confirmed[p.ID]; ok {
			confirmedNonWinners++
		}
	}

	expectedConfirmations := activePlayers - 1
	if expectedConfirmations < 0 {
		expectedConfirmations = 0
	}

	if confirmedNonWinners < expectedConfirmations {
		return nil
	}

	// Todos confirmaram — aplica pontos e finaliza rodada
	applyScore := func(playerID string, points int) {
		if err := database.UpdatePlayerScore(playerID, points); err != nil {
			log.Printf("UpdatePlayerScore error for player %s: %v", playerID, err)
			return
		}
		if updated, err := database.GetPlayer(playerID); err == nil && updated.TotalScore > 51 {
			if err2 := database.UpdatePlayerStatus(playerID, "estourou"); err2 != nil {
				log.Printf("UpdatePlayerStatus error: %v", err2)
			}
			hub.Broadcast(matchID, "player_estourou")
		}
	}

	for _, p := range players {
		if p.Status != "active" {
			continue
		}
		hi, ok := confirmed[p.ID]
		if !ok {
			continue
		}
		if p.ID == round.WinnerPlayerID {
			// Fechamento: vencedor pontuou se trouxe peças na mão
			if hi.PointsCalculated > 0 {
				applyScore(p.ID, hi.PointsCalculated)
			}
			continue
		}
		applyScore(p.ID, hi.PointsCalculated)
	}

	if err := database.FinishRound(roundID); err != nil {
		return err
	}

	hub.Broadcast(matchID, "points_updated")

	players, _ = database.GetPlayers(matchID)
	activePlayers = 0
	for _, p := range players {
		if p.Status == "active" {
			activePlayers++
		}
	}

	if activePlayers <= 1 {
		if err := database.UpdateMatchStatus(matchID, "finished"); err != nil {
			log.Printf("UpdateMatchStatus finished error: %v", err)
			return err
		}
		if allPlayers, err := database.GetPlayers(matchID); err == nil {
			if winnerID := findMatchWinner(allPlayers); winnerID != "" {
				if err2 := database.SetMatchWinner(matchID, winnerID); err2 != nil {
					log.Printf("SetMatchWinner error: %v", err2)
				}
			}
		}
		hub.Broadcast(matchID, "match_finished")
	}

	return nil
}
