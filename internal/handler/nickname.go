package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/leandrodaf/domino-placar/internal/db"

	"github.com/google/uuid"
)

// NicknamePageHandler — GET /match/{id}/nicknames
func NicknamePageHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")

		match, err := database.GetMatch(matchID)
		if err != nil {
			http.Error(w, "match not found", http.StatusNotFound)
			return
		}

		players, _ := database.GetPlayers(matchID)
		nominations, _ := database.GetNominationsForMatch(matchID)

		type PlayerWithNick struct {
			ID               string
			Name             string
			TopNickname      string
			UniqueIdentifier string
		}

		var rows []PlayerWithNick
		for _, p := range players {
			top := ""
			for _, n := range nominations {
				if n.NominatedUniqueID == p.UniqueIdentifier {
					top = n.Nickname
					break
				}
			}
			rows = append(rows, PlayerWithNick{
				ID:               p.ID,
				Name:             p.Name,
				TopNickname:      top,
				UniqueIdentifier: p.UniqueIdentifier,
			})
		}

		// Voter ID vem do cookie de jogador (não exposto diretamente ao usuário)
		voterID := r.URL.Query().Get("voter_id")

		tmpl.Render(w, "nicknames.html", map[string]any{
			"Match":       match,
			"Players":     players,
			"Nominations": nominations,
			"VoterID":     voterID,
			"PlayerRows":  rows,
			"CSRFToken":   GenerateCSRFToken(matchID),
		})
	}
}

// NominateHandler — POST /match/{id}/player/{pid}/nominate
func NominateHandler(database db.Store, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		playerID := r.PathValue("pid")

		if !CheckActionRateLimit(r) {
			http.Error(w, "muitas requisições, aguarde", http.StatusTooManyRequests)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "form inválido", http.StatusBadRequest)
			return
		}

		if !ValidateCSRFToken(r.FormValue("_csrf"), matchID) {
			http.Error(w, "token de segurança inválido", http.StatusForbidden)
			return
		}

		nickname := SanitizeInput(r.FormValue("nickname"), 30)
		voterUID := SanitizeInput(r.FormValue("voter_id"), 50)

		if nickname == "" || voterUID == "" {
			http.Error(w, "apelido e identificador são obrigatórios", http.StatusBadRequest)
			return
		}

		player, err := database.GetPlayer(playerID)
		if err != nil || player.MatchID != matchID {
			http.Error(w, "jogador não encontrado", http.StatusNotFound)
			return
		}

		existing, _ := database.GetNominationsForPlayer(matchID, player.UniqueIdentifier)
		for _, n := range existing {
			if strings.EqualFold(n.Nickname, nickname) {
				database.VoteForNomination(n.ID, voterUID)
				hub.Broadcast(matchID, "nickname_updated")
				http.Redirect(w, r, "/match/"+matchID+"/nicknames?voter_id="+voterUID, http.StatusSeeOther)
				return
			}
		}

		nomID := uuid.New().String()
		if err := database.CreateNomination(nomID, player.UniqueIdentifier, player.Name, matchID, nickname, voterUID); err != nil {
			log.Printf("CreateNomination error: %v", err)
			http.Error(w, "falha ao criar apelido", http.StatusInternalServerError)
			return
		}

		database.VoteForNomination(nomID, voterUID)
		hub.Broadcast(matchID, "nickname_updated")

		http.Redirect(w, r, "/match/"+matchID+"/nicknames?voter_id="+voterUID, http.StatusSeeOther)
	}
}

// VoteNicknameHandler — POST /match/{id}/nickname/{nid}/vote
func VoteNicknameHandler(database db.Store, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		nominationID := r.PathValue("nid")

		if !CheckActionRateLimit(r) {
			http.Error(w, "muitas requisições, aguarde", http.StatusTooManyRequests)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "form inválido", http.StatusBadRequest)
			return
		}

		if !ValidateCSRFToken(r.FormValue("_csrf"), matchID) {
			http.Error(w, "token de segurança inválido", http.StatusForbidden)
			return
		}

		voterUID := SanitizeInput(r.FormValue("voter_id"), 50)
		if voterUID == "" {
			http.Error(w, "identificador obrigatório", http.StatusBadRequest)
			return
		}

		voted, err := database.VoteForNomination(nominationID, voterUID)
		if err != nil {
			log.Printf("VoteForNomination error: %v", err)
			http.Error(w, "falha ao votar", http.StatusInternalServerError)
			return
		}
		_ = voted

		hub.Broadcast(matchID, "nickname_updated")
		http.Redirect(w, r, "/match/"+matchID+"/nicknames?voter_id="+voterUID, http.StatusSeeOther)
	}
}
