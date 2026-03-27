package handler

import (
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"

	"github.com/leandrodaf/domino-placar/internal/db"
	"github.com/leandrodaf/domino-placar/internal/models"
)

type Templates struct {
	funcMap template.FuncMap
}

func NewTemplates() (*Templates, error) {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"mul": func(a, b int) int { return a * b },
		"sub": func(a, b int) int { return a - b },
	}
	_, err := template.New("").Funcs(funcMap).ParseGlob("templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Templates{funcMap: funcMap}, nil
}

func (tmpl *Templates) Render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t, err := template.New("").Funcs(tmpl.funcMap).ParseFiles("templates/base.html", "templates/"+name)
	if err != nil {
		log.Printf("template parse %s error: %v", name, err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("template exec %s error: %v", name, err)
	}
}

func (tmpl *Templates) RenderPartial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t, err := template.New("").Funcs(tmpl.funcMap).ParseFiles("templates/" + name)
	if err != nil {
		log.Printf("partial parse %s error: %v", name, err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("partial exec %s error: %v", name, err)
	}
}

func uploadErrorMsg(code string) string {
	switch code {
	case "too_large":
		return "Arquivo muito grande. O tamanho máximo permitido é 5 MB."
	case "invalid_type":
		return "Tipo de arquivo não suportado. Envie uma imagem JPEG ou PNG."
	case "rate_limit":
		return "Muitos envios seguidos. Aguarde alguns minutos e tente novamente."
	case "invalid_points":
		return "Pontuação inválida. Digite um número inteiro entre 0 e 200."
	case "read_failed":
		return "Não foi possível ler o arquivo. Tente novamente."
	case "invalid_form":
		return "Formulário inválido. Tente novamente."
	}
	return ""
}

func HomeHandler(tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		tmpl.Render(w, "home.html", nil)
	}
}

// MatchResumeHandler — GET /match/{id}
func MatchResumeHandler(database db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		match, err := database.GetMatch(matchID)
		if err != nil {
			http.Error(w, "partida não encontrada", http.StatusNotFound)
			return
		}
		switch match.Status {
		case "waiting":
			http.Redirect(w, r, "/match/"+matchID+"/lobby", http.StatusSeeOther)
		case "active":
			round, err := database.GetCurrentRound(matchID)
			if err == nil {
				http.Redirect(w, r, "/match/"+matchID+"/round/"+round.ID+"/game", http.StatusSeeOther)
			} else {
				http.Redirect(w, r, "/match/"+matchID+"/ranking", http.StatusSeeOther)
			}
		default:
			http.Redirect(w, r, "/match/"+matchID+"/ranking", http.StatusSeeOther)
		}
	}
}

func LobbyHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		match, err := database.GetMatch(matchID)
		if err != nil {
			http.Error(w, "match not found", http.StatusNotFound)
			return
		}
		players, err := database.GetPlayers(matchID)
		if err != nil {
			players = []models.Player{}
		}

		tiles := CalcTiles(len(players))
		tmpl.Render(w, "lobby.html", map[string]any{
			"Match":     match,
			"Players":   players,
			"Count":     len(players),
			"Tiles":     tiles,
			"CSRFToken": GenerateCSRFToken(matchID),
		})
	}
}

func PlayersPartialHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		players, err := database.GetPlayers(matchID)
		if err != nil {
			players = []models.Player{}
		}
		tiles := CalcTiles(len(players))
		tmpl.RenderPartial(w, "players-partial.html", map[string]any{
			"Players":   players,
			"Count":     len(players),
			"MatchID":   matchID,
			"Tiles":     tiles,
			"CSRFToken": GenerateCSRFToken(matchID),
		})
	}
}

func JoinPageHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		match, err := database.GetMatch(matchID)
		if err != nil {
			http.Error(w, "match not found", http.StatusNotFound)
			return
		}
		if match.Status != "waiting" {
			tmpl.Render(w, "join.html", map[string]any{
				"Match": match,
				"Error": "Esta partida já foi iniciada.",
			})
			return
		}

		var errMsg string
		switch r.URL.Query().Get("error") {
		case "full":
			errMsg = "Esta partida está cheia (máximo 10 jogadores)."
		case "name_taken":
			errMsg = "Este nome já está sendo usado por outro jogador. Escolha um nome diferente."
		}

		tmpl.Render(w, "join.html", map[string]any{"Match": match, "Error": errMsg})
	}
}

func WaitingHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		match, err := database.GetMatch(matchID)
		if err != nil {
			http.Error(w, "match not found", http.StatusNotFound)
			return
		}

		playerID := r.URL.Query().Get("player_id")

		if match.Status == "active" {
			pidParam := ""
			if playerID != "" {
				pidParam = "?pid=" + playerID
			}
			round, err := database.GetCurrentRound(matchID)
			if err == nil {
				http.Redirect(w, r, "/match/"+matchID+"/round/"+round.ID+"/game"+pidParam, http.StatusSeeOther)
			} else {
				http.Redirect(w, r, "/match/"+matchID+"/ranking"+pidParam, http.StatusSeeOther)
			}
			return
		}
		if match.Status == "finished" {
			pidParam := ""
			if playerID != "" {
				pidParam = "?pid=" + playerID
			}
			http.Redirect(w, r, "/match/"+matchID+"/ranking"+pidParam, http.StatusSeeOther)
			return
		}
		var player *models.Player
		if playerID != "" {
			player, _ = database.GetPlayer(playerID)
		}

		tmpl.Render(w, "waiting.html", map[string]any{
			"Match":  match,
			"Player": player,
		})
	}
}

func GameHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		roundID := r.PathValue("rid")

		match, err := database.GetMatch(matchID)
		if err != nil {
			http.Error(w, "match not found", http.StatusNotFound)
			return
		}

		round, err := database.GetRound(roundID)
		if err != nil || round.MatchID != matchID {
			http.Error(w, "round not found", http.StatusNotFound)
			return
		}

		players, err := database.GetPlayers(matchID)
		if err != nil {
			players = []models.Player{}
		}

		handImages, err := database.GetHandImages(roundID)
		if err != nil {
			handImages = []models.HandImage{}
		}

		submitted := map[string]bool{}
		for _, hi := range handImages {
			submitted[hi.PlayerID] = true
		}

		confirmed := map[string]bool{}
		for _, hi := range handImages {
			if hi.Confirmed == 1 {
				confirmed[hi.PlayerID] = true
			}
		}

		type SeatRow struct {
			Player    models.Player
			Submitted bool
			Confirmed bool
			IsWinner  bool
			IsLeader  bool
			SeatX     string
			SeatY     string
		}

		// Determina o líder: jogador ativo com menor pontuação
		leaderID := ""
		leaderScore := 999
		for _, p := range players {
			if p.Status == "active" && p.TotalScore < leaderScore {
				leaderScore = p.TotalScore
				leaderID = p.ID
			}
		}

		n := len(players)
		mesaType := "round"
		if n <= 2 {
			mesaType = "rect"
		}

		var seatRows []SeatRow
		for i, p := range players {
			var sx, sy float64
			switch {
			case n == 1:
				sx, sy = 50, 50
			case n == 2:
				if i == 0 {
					sx, sy = 50, 11
				} else {
					sx, sy = 50, 89
				}
			default:
				radius := 37.0
				if n >= 7 {
					radius = 39.0
				}
				angle := 2*math.Pi*float64(i)/float64(n) - math.Pi/2
				sx = 50 + radius*math.Cos(angle)
				sy = 50 + radius*math.Sin(angle)
			}
			seatRows = append(seatRows, SeatRow{
				Player:    p,
				Submitted: submitted[p.ID],
				Confirmed: confirmed[p.ID],
				IsWinner:  p.ID == round.WinnerPlayerID,
				IsLeader:  p.ID == leaderID,
				SeatX:     fmt.Sprintf("%.1f", sx),
				SeatY:     fmt.Sprintf("%.1f", sy),
			})
		}

		type PlayerRow struct {
			Player    models.Player
			Submitted bool
			Confirmed bool
		}
		var playerRows []PlayerRow
		for _, p := range players {
			if p.Status == "active" {
				playerRows = append(playerRows, PlayerRow{
					Player:    p,
					Submitted: submitted[p.ID],
					Confirmed: confirmed[p.ID],
				})
			}
		}

		tiles := CalcTiles(len(players))

		_, tableJSON, _ := database.GetRoundTableTiles(roundID)
		tileStats := BuildTileStats(handImages, tableJSON, tiles.TotalTiles)

		// Determina se o visitante é host ou jogador
		isHost := IsHost(r, matchID)
		currentPlayerID := ""
		if !isHost {
			if pid := r.URL.Query().Get("pid"); pid != "" {
				if p, err2 := database.GetPlayer(pid); err2 == nil && p.MatchID == matchID {
					currentPlayerID = pid
				}
			}
			// Fallback: verifica cookie de jogador
			if currentPlayerID == "" {
				currentPlayerID = GetAuthPlayerID(r, matchID)
			}
		}

		tmpl.Render(w, "game.html", map[string]any{
			"Match":           match,
			"Round":           round,
			"PlayerRows":      playerRows,
			"SeatRows":        seatRows,
			"MesaType":        mesaType,
			"Tiles":           tiles,
			"TileStats":       tileStats,
			"ErrorMsg":        uploadErrorMsg(r.URL.Query().Get("error")),
			"IsHost":          isHost,
			"CurrentPlayerID": currentPlayerID,
			"CSRFToken":       GenerateCSRFToken(matchID),
		})
	}
}

// UploadPageHandler serves the upload page for a specific player.
func UploadPageHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		roundID := r.PathValue("rid")
		playerID := r.PathValue("pid")

		// Autenticação: apenas o próprio jogador ou o anfitrião
		authPID := GetAuthPlayerID(r, matchID)
		if authPID != playerID && !IsHost(r, matchID) {
			http.Redirect(w, r, "/match/"+matchID+"/join", http.StatusSeeOther)
			return
		}

		match, err := database.GetMatch(matchID)
		if err != nil {
			http.Error(w, "match not found", http.StatusNotFound)
			return
		}
		round, err := database.GetRound(roundID)
		if err != nil || round.MatchID != matchID {
			http.Error(w, "round not found", http.StatusNotFound)
			return
		}
		player, err := database.GetPlayer(playerID)
		if err != nil || player.MatchID != matchID {
			http.Error(w, "player not found", http.StatusNotFound)
			return
		}

		existing, _ := database.GetHandImageByRoundAndPlayer(roundID, playerID)
		if existing != nil {
			http.Redirect(w, r, "/match/"+matchID+"/round/"+roundID+"/confirm/"+playerID, http.StatusSeeOther)
			return
		}

		players, _ := database.GetPlayers(matchID)
		tiles := CalcTiles(len(players))

		tmpl.Render(w, "upload.html", map[string]any{
			"Match":     match,
			"Round":     round,
			"Player":    player,
			"Tiles":     tiles,
			"ErrorMsg":  uploadErrorMsg(r.URL.Query().Get("error")),
			"CSRFToken": GenerateCSRFToken(matchID),
		})
	}
}

// ConfirmPageHandler serves the confirmation page after CV analysis.
func ConfirmPageHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		roundID := r.PathValue("rid")
		playerID := r.PathValue("pid")

		// Autenticação: apenas o próprio jogador ou o anfitrião
		authPID := GetAuthPlayerID(r, matchID)
		if authPID != playerID && !IsHost(r, matchID) {
			http.Redirect(w, r, "/match/"+matchID+"/join", http.StatusSeeOther)
			return
		}

		match, err := database.GetMatch(matchID)
		if err != nil {
			http.Error(w, "match not found", http.StatusNotFound)
			return
		}
		round, err := database.GetRound(roundID)
		if err != nil || round.MatchID != matchID {
			http.Error(w, "round not found", http.StatusNotFound)
			return
		}
		player, err := database.GetPlayer(playerID)
		if err != nil || player.MatchID != matchID {
			http.Error(w, "player not found", http.StatusNotFound)
			return
		}

		handImage, err := database.GetHandImageByRoundAndPlayer(roundID, playerID)
		if err != nil {
			http.Redirect(w, r, "/match/"+matchID+"/round/"+roundID+"/upload/"+playerID, http.StatusSeeOther)
			return
		}

		cvDetected := handImage.PointsCalculated >= 0
		manualMode := handImage.PointsCalculated == -1

		tmpl.Render(w, "confirm.html", map[string]any{
			"Match":      match,
			"Round":      round,
			"Player":     player,
			"HandImage":  handImage,
			"CVDetected": cvDetected,
			"ManualMode": manualMode,
			"CSRFToken":  GenerateCSRFToken(matchID),
		})
	}
}

func GlobalRankingHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := database.GetGlobalStats()
		if err != nil {
			log.Printf("GetGlobalStats error: %v", err)
			stats = nil
		}

		mostLost, _ := database.GetMostRoundsLost()
		pintoKings, _ := database.GetPintoKings()
		brancoKings, _ := database.GetBrancoKings()
		closeCallKings, _ := database.GetCloseCallKings()
		allTimeNicks, _ := database.GetAllTimeNicknames()

		tmpl.Render(w, "global-ranking.html", map[string]any{
			"Stats":          stats,
			"MostLost":       mostLost,
			"PintoKings":     pintoKings,
			"BrancoKings":    brancoKings,
			"CloseCallKings": closeCallKings,
			"AllTimeNicks":   allTimeNicks,
		})
	}
}

func RankingHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")

		match, err := database.GetMatch(matchID)
		if err != nil {
			http.Error(w, "match not found", http.StatusNotFound)
			return
		}

		players, err := database.GetRanking(matchID)
		if err != nil {
			players = []models.Player{}
		}

		var round *models.Round
		roundCount := 0
		if match.Status == "active" {
			round, _ = database.GetCurrentRound(matchID)
			roundCount, _ = database.CountRounds(matchID)
		}

		nickMap := map[string]string{}
		if noms, err := database.GetNominationsForMatch(matchID); err == nil {
			seen := map[string]bool{}
			for _, n := range noms {
				if !seen[n.NominatedUniqueID] {
					seen[n.NominatedUniqueID] = true
					nickMap[n.NominatedUniqueID] = n.Nickname
				}
			}
		}

		starterName := ""
		if round != nil && round.StarterPlayerID != "" {
			if p, err := database.GetPlayer(round.StarterPlayerID); err == nil {
				starterName = p.Name
			}
		}

		// Determina role: host ou jogador
		isHost := IsHost(r, matchID)
		currentPlayerID := ""
		if !isHost {
			if pid := r.URL.Query().Get("pid"); pid != "" {
				if p, err2 := database.GetPlayer(pid); err2 == nil && p.MatchID == matchID {
					currentPlayerID = pid
				}
			}
			if currentPlayerID == "" {
				currentPlayerID = GetAuthPlayerID(r, matchID)
			}
		}

		tmpl.Render(w, "ranking.html", map[string]any{
			"Match":           match,
			"Players":         players,
			"Round":           round,
			"RoundCount":      roundCount,
			"NickMap":         nickMap,
			"StarterName":     starterName,
			"IsHost":          isHost,
			"CurrentPlayerID": currentPlayerID,
			"CSRFToken":       GenerateCSRFToken(matchID),
		})
	}
}
