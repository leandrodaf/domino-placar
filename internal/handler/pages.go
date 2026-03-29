package handler

import (
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/leandrodaf/domino-placar/internal/db"
	"github.com/leandrodaf/domino-placar/internal/i18n"
	"github.com/leandrodaf/domino-placar/internal/models"
)

type Templates struct {
	pages    map[string]*template.Template // page name -> base+page (pre-parsed)
	partials map[string]*template.Template // partial name -> partial (pre-parsed)
}

func NewTemplates() (*Templates, error) {
	funcMap := template.FuncMap{
		"add":      func(a, b int) int { return a + b },
		"mul":      func(a, b int) int { return a * b },
		"sub":      func(a, b int) int { return a - b },
		"T":        i18n.T,
		"TH":       i18n.TH,
		"LangAttr": i18n.LangHTMLAttr,
	}

	pages := make(map[string]*template.Template)
	partials := make(map[string]*template.Template)

	files, err := filepath.Glob("templates/*.html")
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		name := filepath.Base(f)
		if name == "base.html" {
			continue
		}

		if strings.Contains(name, "partial") {
			t, err := template.New("").Funcs(funcMap).ParseFiles(f)
			if err != nil {
				return nil, fmt.Errorf("parsing partial %s: %w", name, err)
			}
			partials[name] = t
		} else {
			t, err := template.New("").Funcs(funcMap).ParseFiles("templates/base.html", f)
			if err != nil {
				return nil, fmt.Errorf("parsing page %s: %w", name, err)
			}
			pages[name] = t
		}
	}

	// Validate at least base.html exists
	if _, err := os.Stat("templates/base.html"); err != nil {
		return nil, fmt.Errorf("base.html not found: %w", err)
	}

	return &Templates{pages: pages, partials: partials}, nil
}

func (tmpl *Templates) Render(w http.ResponseWriter, r *http.Request, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t, ok := tmpl.pages[name]
	if !ok {
		log.Printf("template %s not found in cache", name)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	data = injectLang(data, i18n.DetectLang(r))
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("template exec %s error: %v", name, err)
	}
}

func (tmpl *Templates) RenderPartial(w http.ResponseWriter, r *http.Request, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t, ok := tmpl.partials[name]
	if !ok {
		log.Printf("partial %s not found in cache", name)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	data = injectLang(data, i18n.DetectLang(r))
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("partial exec %s error: %v", name, err)
	}
}

// injectLang adds the Lang key to the template data map.
func injectLang(data any, lang string) any {
	if data == nil {
		return map[string]any{"Lang": lang}
	}
	if m, ok := data.(map[string]any); ok {
		m["Lang"] = lang
		return m
	}
	return data
}

func uploadErrorMsg(code string) string {
	switch code {
	case "too_large":
		return "File too large. Maximum allowed size is 5 MB."
	case "invalid_type":
		return "Unsupported file type. Please upload a JPEG or PNG image."
	case "rate_limit":
		return "Too many uploads in a row. Please wait a few minutes and try again."
	case "invalid_points":
		return "Invalid score. Enter an integer between 0 and 200."
	case "read_failed":
		return "Could not read the file. Please try again."
	case "invalid_form":
		return "Invalid form. Please try again."
	}
	return ""
}

func HomeHandler(tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		tmpl.Render(w, r, "home.html", nil)
	}
}

// MatchResumeHandler — GET /match/{id}
func MatchResumeHandler(database db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		match, err := database.GetMatch(matchID)
		if err != nil {
			http.Error(w, "match not found", http.StatusNotFound)
			return
		}
		switch match.Status {
		case "waiting":
			if IsHost(r, matchID) {
				http.Redirect(w, r, "/match/"+matchID+"/lobby", http.StatusSeeOther)
			} else if pid := GetAuthPlayerID(r, matchID); pid != "" {
				http.Redirect(w, r, "/match/"+matchID+"/waiting?player_id="+pid, http.StatusSeeOther)
			} else {
				http.Redirect(w, r, "/match/"+matchID+"/join", http.StatusSeeOther)
			}
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

		// Only the host can access the lobby (control panel)
		if !IsHost(r, matchID) {
			if pid := GetAuthPlayerID(r, matchID); pid != "" {
				http.Redirect(w, r, "/match/"+matchID+"/waiting?player_id="+pid, http.StatusSeeOther)
			} else {
				http.Redirect(w, r, "/match/"+matchID+"/join", http.StatusSeeOther)
			}
			return
		}

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
		data := map[string]any{
			"Match":     match,
			"Players":   players,
			"Count":     len(players),
			"Tiles":     tiles,
			"IsHost":    true,
			"CSRFToken": GenerateCSRFToken(matchID),
		}
		if match.TurmaID != "" {
			if turma, err := database.GetTurma(match.TurmaID); err == nil {
				data["Turma"] = turma
			}
		}
		tmpl.Render(w, r, "lobby.html", data)
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
		tmpl.RenderPartial(w, r, "players-partial.html", map[string]any{
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
			tmpl.Render(w, r, "join.html", map[string]any{
				"Match": match,
				"Error": "This match has already started.",
			})
			return
		}

		var errMsg string
		switch r.URL.Query().Get("error") {
		case "full":
			errMsg = "This match is full (maximum 10 players)."
		case "name_taken":
			errMsg = "This name is already taken by another player. Choose a different name."
		}

		tmpl.Render(w, r, "join.html", map[string]any{"Match": match, "Error": errMsg})
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

		tmpl.Render(w, r, "waiting.html", map[string]any{
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

		// Determine the leader: active player with lowest score
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
			switch n {
			case 1:
				sx, sy = 50, 50
			case 2:
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

		// Determine if visitor is host or player
		isHost := IsHost(r, matchID)
		currentPlayerID := ""
		if !isHost {
			if pid := r.URL.Query().Get("pid"); pid != "" {
				if p, err2 := database.GetPlayer(pid); err2 == nil && p.MatchID == matchID {
					currentPlayerID = pid
				}
			}
			// Fallback: check player cookie
			if currentPlayerID == "" {
				currentPlayerID = GetAuthPlayerID(r, matchID)
			}
		}

		tmpl.Render(w, r, "game.html", map[string]any{
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

		// Auth: only the player themselves or the host
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

		tmpl.Render(w, r, "upload.html", map[string]any{
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

		// Auth: only the player themselves or the host
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

		tmpl.Render(w, r, "confirm.html", map[string]any{
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

		tmpl.Render(w, r, "global-ranking.html", map[string]any{
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

		// Determine role: host or player
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

		tmpl.Render(w, r, "ranking.html", map[string]any{
			"Match":           match,
			"Players":         players,
			"Round":           round,
			"RoundCount":      roundCount,
			"NickMap":         nickMap,
			"StarterName":     starterName,
			"IsHost":          isHost,
			"CurrentPlayerID": currentPlayerID,
			"CSRFToken":       GenerateCSRFToken(matchID),
			"TurmaID":         match.TurmaID,
		})
	}
}
