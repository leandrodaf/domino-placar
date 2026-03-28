package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/leandrodaf/domino-placar/internal/db"
	"github.com/leandrodaf/domino-placar/internal/models"
	"github.com/leandrodaf/domino-placar/internal/service"

	"github.com/google/uuid"
)

// saveImage saves imageBytes: tries GCS, falls back to local disk if GCS_BUCKET is not configured.
// Returns the path/URL of the saved file.
func saveImage(imageBytes []byte, objectName string) (string, error) {
	if url, err := service.UploadImageToGCS(imageBytes, objectName); err != nil {
		return "", fmt.Errorf("GCS upload: %w", err)
	} else if url != "" {
		return url, nil
	}
	// Fallback local
	localPath := filepath.Join("uploads", objectName)
	if err := os.WriteFile(localPath, imageBytes, 0644); err != nil {
		return "", fmt.Errorf("saving locally: %w", err)
	}
	return localPath, nil
}

const (
	maxUploadSize = 5 << 20 // 5 MB
	maxScore      = 200     // max allowed score
)

// UploadHandler handles POST /match/{id}/round/{rid}/upload/{pid}
func UploadHandler(database db.Store, hub *SSEHub, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		roundID := r.PathValue("rid")
		playerID := r.PathValue("pid")

		uploadBase := "/match/" + matchID + "/round/" + roundID + "/upload/" + playerID

		// Auth: only the player themselves or the host
		authPID := GetAuthPlayerID(r, matchID)
		if authPID != playerID && !IsHost(r, matchID) {
			http.Redirect(w, r, "/match/"+matchID+"/join", http.StatusSeeOther)
			return
		}

		if !CheckActionRateLimit(r) {
			http.Redirect(w, r, uploadBase+"?error=rate_limit", http.StatusSeeOther)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

		contentType := r.Header.Get("Content-Type")
		if len(contentType) > 9 && contentType[:9] == "multipart" {
			if err := r.ParseMultipartForm(maxUploadSize); err != nil {
				http.Redirect(w, r, uploadBase+"?error=too_large", http.StatusSeeOther)
				return
			}
		} else {
			if err := r.ParseForm(); err != nil {
				http.Redirect(w, r, uploadBase+"?error=invalid_form", http.StatusSeeOther)
				return
			}
		}

		if !ValidateCSRFToken(r.FormValue("_csrf"), matchID) {
			http.Error(w, "invalid security token", http.StatusForbidden)
			return
		}

		player, err := database.GetPlayer(playerID)
		if err != nil || player.MatchID != matchID {
			http.Error(w, "player not found", http.StatusNotFound)
			return
		}

		// Manual point entry (no photo)
		manualPoints := r.FormValue("manual_points")
		if manualPoints != "" {
			pts, err := strconv.Atoi(manualPoints)
			if err != nil || pts < 0 || pts > maxScore {
				http.Redirect(w, r, uploadBase+"?error=invalid_points", http.StatusSeeOther)
				return
			}

			imageID := uuid.New().String()
			if err := database.CreateHandImage(imageID, roundID, playerID, "manual:"+imageID); err != nil {
				log.Printf("CreateHandImage error: %v", err)
				http.Error(w, "failed to save image record", http.StatusInternalServerError)
				return
			}
			if err := database.UpdateHandImagePoints(imageID, pts, false, "[]"); err != nil {
				log.Printf("UpdateHandImagePoints error: %v", err)
			}

			http.Redirect(w, r, "/match/"+matchID+"/round/"+roundID+"/confirm/"+playerID, http.StatusSeeOther)
			return
		}

		// Photo-specific rate limit
		if !CheckUploadRateLimit(r) {
			http.Redirect(w, r, uploadBase+"?error=rate_limit", http.StatusSeeOther)
			return
		}

		file, _, err := r.FormFile("image")
		if err != nil {
			http.Redirect(w, r, uploadBase, http.StatusSeeOther)
			return
		}
		defer file.Close()

		imageBytes, err := io.ReadAll(file)
		if err != nil {
			log.Printf("ReadAll error: %v", err)
			http.Redirect(w, r, uploadBase+"?error=read_failed", http.StatusSeeOther)
			return
		}

		if err := service.ValidateImageMIME(imageBytes); err != nil {
			http.Redirect(w, r, uploadBase+"?error=invalid_type", http.StatusSeeOther)
			return
		}

		compressed, err := service.CompressImage(imageBytes)
		if err != nil {
			log.Printf("CompressImage error: %v, saving original", err)
			compressed = imageBytes
		}

		imageID := uuid.New().String()
		imagePath, err := saveImage(compressed, imageID+".jpg")
		if err != nil {
			log.Printf("saveImage error: %v", err)
			http.Error(w, "failed to save image", http.StatusInternalServerError)
			return
		}

		if err := database.CreateHandImage(imageID, roundID, playerID, imagePath); err != nil {
			log.Printf("CreateHandImage error: %v", err)
			http.Error(w, "failed to save image record", http.StatusInternalServerError)
			return
		}

		// Points will be entered manually by the player on the confirmation page
		if err := database.UpdateHandImagePoints(imageID, -1, false, "[]"); err != nil {
			log.Printf("UpdateHandImagePoints error: %v", err)
		}

		http.Redirect(w, r, "/match/"+matchID+"/round/"+roundID+"/confirm/"+playerID, http.StatusSeeOther)
	}
}

// TableImageHandler handles POST /match/{id}/round/{rid}/table-image — table photo by the host.
func TableImageHandler(database db.Store, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		roundID := r.PathValue("rid")

		gameBase := "/match/" + matchID + "/round/" + roundID + "/game"

		if !IsHost(r, matchID) {
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}
		if !CheckActionRateLimit(r) {
			http.Redirect(w, r, gameBase+"?error=rate_limit", http.StatusSeeOther)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			http.Redirect(w, r, gameBase+"?error=too_large", http.StatusSeeOther)
			return
		}

		if !ValidateCSRFToken(r.FormValue("_csrf"), matchID) {
			http.Error(w, "invalid security token", http.StatusForbidden)
			return
		}

		if !CheckUploadRateLimit(r) {
			http.Redirect(w, r, gameBase+"?error=rate_limit", http.StatusSeeOther)
			return
		}

		file, _, err := r.FormFile("image")
		if err != nil {
			http.Redirect(w, r, gameBase, http.StatusSeeOther)
			return
		}
		defer file.Close()

		imageBytes, err := io.ReadAll(file)
		if err != nil {
			log.Printf("ReadAll error: %v", err)
			http.Redirect(w, r, gameBase+"?error=read_failed", http.StatusSeeOther)
			return
		}

		if err := service.ValidateImageMIME(imageBytes); err != nil {
			http.Redirect(w, r, gameBase+"?error=invalid_type", http.StatusSeeOther)
			return
		}

		compressed, err := service.CompressImage(imageBytes)
		if err != nil {
			log.Printf("CompressImage (table) error: %v", err)
			compressed = imageBytes
		}

		imageID := uuid.New().String()
		imagePath, err := saveImage(compressed, "table_"+imageID+".jpg")
		if err != nil {
			log.Printf("saveImage (table) error: %v", err)
			http.Error(w, "failed to save image", http.StatusInternalServerError)
			return
		}

		// Save the table photo (no automatic AI analysis)
		if err := database.SetTableImage(roundID, imagePath, "[]"); err != nil {
			log.Printf("SetTableImage error: %v", err)
		}

		hub.Broadcast(matchID, "table_image_updated")
		http.Redirect(w, r, "/match/"+matchID+"/round/"+roundID+"/game", http.StatusSeeOther)
	}
}

// ConfirmHandler handles POST /match/{id}/round/{rid}/confirm/{pid}
func ConfirmHandler(database db.Store, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		roundID := r.PathValue("rid")
		playerID := r.PathValue("pid")

		// Auth: only the player themselves or the host
		authPID := GetAuthPlayerID(r, matchID)
		if authPID != playerID && !IsHost(r, matchID) {
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}
		if !CheckActionRateLimit(r) {
			http.Error(w, "too many requests, please wait", http.StatusTooManyRequests)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		if !ValidateCSRFToken(r.FormValue("_csrf"), matchID) {
			http.Error(w, "invalid security token", http.StatusForbidden)
			return
		}

		// Validate that the player belongs to this match
		player, err := database.GetPlayer(playerID)
		if err != nil || player.MatchID != matchID {
			http.Error(w, "player not found", http.StatusNotFound)
			return
		}

		handImage, err := database.GetHandImageByRoundAndPlayer(roundID, playerID)
		if err != nil {
			http.Error(w, "hand image not found", http.StatusNotFound)
			return
		}

		overrideStr := r.FormValue("override_points")
		points := handImage.PointsCalculated
		if overrideStr != "" {
			override, err := strconv.Atoi(overrideStr)
			if err == nil && override >= 0 && override <= maxScore {
				points = override
			}
		}
		// Without AI, negative points are not valid — override required
		if points < 0 {
			http.Error(w, "score required", http.StatusBadRequest)
			return
		}

		tiles := unmarshalTiles(handImage.DetectedTilesJSON)
		points = service.ApplySpecialRules(tiles, points)

		if err := database.UpdateHandImagePoints(handImage.ID, points, true, handImage.DetectedTilesJSON); err != nil {
			log.Printf("UpdateHandImagePoints error: %v", err)
			http.Error(w, "failed to confirm points", http.StatusInternalServerError)
			return
		}

		hub.Broadcast(matchID, "points_updated")

		if err := checkRoundComplete(database, hub, matchID, roundID); err != nil {
			log.Printf("checkRoundComplete error: %v", err)
		}

		http.Redirect(w, r, "/match/"+matchID+"/round/"+roundID+"/ranking", http.StatusSeeOther)
	}
}

// JoinHandler handles POST /match/{id}/join — registers a player.
func JoinHandler(database db.Store, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")

		if !CheckActionRateLimit(r) {
			http.Error(w, "too many requests, please wait", http.StatusTooManyRequests)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		name := SanitizeInput(r.FormValue("name"), 50)
		uniqueID := SanitizeInput(r.FormValue("unique_id"), 50)

		if name == "" || uniqueID == "" {
			http.Error(w, "name and identifier are required", http.StatusBadRequest)
			return
		}

		match, err := database.GetMatch(matchID)
		if err != nil {
			http.Error(w, "match not found", http.StatusNotFound)
			return
		}

		if match.Status != "waiting" {
			http.Error(w, "match already started", http.StatusBadRequest)
			return
		}

		count, err := database.CountPlayersByMatch(matchID)
		if err == nil && count >= 10 {
			http.Redirect(w, r, "/match/"+matchID+"/join?error=full", http.StatusSeeOther)
			return
		}

		redirectTo := r.FormValue("redirect_to")
		if redirectTo == "" || !strings.HasPrefix(redirectTo, "/") {
			redirectTo = ""
		}

		existing, err := database.GetPlayerByUniqueID(matchID, uniqueID)
		if err == nil && existing != nil {
			// Player already exists — update name if changed
			if existing.Name != name {
				if err := database.UpdatePlayerName(existing.ID, name); err != nil {
					log.Printf("UpdatePlayerName error: %v", err)
				}
			}
			SetPlayerCookie(w, matchID, existing.ID)
			dest := "/match/" + matchID + "/waiting?player_id=" + existing.ID
			if redirectTo != "" {
				dest = redirectTo
			}
			http.Redirect(w, r, dest, http.StatusSeeOther)
			return
		}

		// Check if the name is already taken by another player
		players, _ := database.GetPlayers(matchID)
		for _, p := range players {
			if strings.EqualFold(p.Name, name) {
				http.Redirect(w, r, "/match/"+matchID+"/join?error=name_taken", http.StatusSeeOther)
				return
			}
		}

		playerID := uuid.New().String()
		if err := database.CreatePlayer(playerID, matchID, name, uniqueID); err != nil {
			log.Printf("CreatePlayer error: %v", err)
			http.Error(w, "failed to join match", http.StatusInternalServerError)
			return
		}

		// Set the player authentication cookie
		SetPlayerCookie(w, matchID, playerID)

		hub.Broadcast(matchID, "player_joined:"+name)

		dest := "/match/" + matchID + "/waiting?player_id=" + playerID
		if redirectTo != "" {
			dest = redirectTo
		}
		http.Redirect(w, r, dest, http.StatusSeeOther)
	}
}

// RoundScoresPageHandler handles GET /match/{id}/round/{rid}/round-scores — bulk score entry page.
func RoundScoresPageHandler(database db.Store, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		roundID := r.PathValue("rid")

		if !IsHost(r, matchID) {
			http.Redirect(w, r, "/match/"+matchID+"/ranking", http.StatusSeeOther)
			return
		}

		match, err := database.GetMatch(matchID)
		if err != nil {
			http.Error(w, "match not found", http.StatusNotFound)
			return
		}
		round, err := database.GetRound(roundID)
		if err != nil {
			http.Error(w, "round not found", http.StatusNotFound)
			return
		}

		players, err := database.GetPlayers(matchID)
		if err != nil {
			players = []models.Player{}
		}

		// Load already confirmed scores to pre-fill
		confirmed := map[string]int{}
		if images, err := database.GetHandImages(roundID); err == nil {
			for _, hi := range images {
				if hi.Confirmed == 1 {
					confirmed[hi.PlayerID] = hi.PointsCalculated
				}
			}
		}

		var errMsg string
		if r.URL.Query().Get("error") == "incomplete" {
			errMsg = "Please confirm all player scores before starting the next round."
		}
		fechamento := r.URL.Query().Get("fechamento") == "1" || round.WinnerPlayerID == ""

		tmpl.Render(w, "round-scores.html", map[string]any{
			"Match":      match,
			"Round":      round,
			"Players":    players,
			"Confirmed":  confirmed,
			"CSRFToken":  GenerateCSRFToken(matchID),
			"ErrorMsg":   errMsg,
			"Fechamento": fechamento,
		})
	}
}

// BulkScoreHandler handles POST /match/{id}/round/{rid}/round-scores — saves all scores at once.
func BulkScoreHandler(database db.Store, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		roundID := r.PathValue("rid")

		if !IsHost(r, matchID) {
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}
		if !CheckActionRateLimit(r) {
			http.Error(w, "too many requests, please wait", http.StatusTooManyRequests)
			return
		}
		if err := r.ParseForm(); err != nil || !ValidateCSRFToken(r.FormValue("_csrf"), matchID) {
			http.Error(w, "invalid security token", http.StatusForbidden)
			return
		}

		round, err := database.GetRound(roundID)
		if err != nil {
			http.Error(w, "round not found", http.StatusNotFound)
			return
		}

		players, err := database.GetPlayers(matchID)
		if err != nil {
			http.Error(w, "failed to load players", http.StatusInternalServerError)
			return
		}

		for _, p := range players {
			if p.Status != "active" {
				continue
			}
			scoreStr := r.FormValue("score_" + p.ID)
			if scoreStr == "" {
				continue
			}
			pts, err := strconv.Atoi(scoreStr)
			if err != nil || pts < 0 || pts > maxScore {
				continue
			}

			// Winner with empty score in form already handled above (scoreStr == "")
			// Create or reuse HandImage for this player
			prefix := "bulk"
			if p.ID == round.WinnerPlayerID {
				prefix = "winner"
			}
			existing, _ := database.GetHandImageByRoundAndPlayer(roundID, p.ID)
			var imageID string
			if existing != nil {
				imageID = existing.ID
			} else {
				imageID = uuid.New().String()
				if err := database.CreateHandImage(imageID, roundID, p.ID, prefix+":"+imageID); err != nil {
					log.Printf("CreateHandImage bulk error: %v", err)
					continue
				}
			}
			if err := database.UpdateHandImagePoints(imageID, pts, true, "[]"); err != nil {
				log.Printf("UpdateHandImagePoints bulk error: %v", err)
			}
		}

		// Closing: if there is no winner, elect the active player with the lowest submitted score
		if round.WinnerPlayerID == "" && r.FormValue("fechamento") == "1" {
			bestPID := ""
			bestScore := maxScore + 1
			for _, p := range players {
				if p.Status != "active" {
					continue
				}
				s := r.FormValue("score_" + p.ID)
				if pts, err := strconv.Atoi(s); err == nil && pts < bestScore {
					bestScore = pts
					bestPID = p.ID
				}
			}
			if bestPID != "" {
				if err := database.SetRoundWinner(roundID, bestPID); err != nil {
					log.Printf("SetRoundWinner fechamento error: %v", err)
				} else {
					round.WinnerPlayerID = bestPID
					hub.Broadcast(matchID, "round_winner_set:"+bestPID)
				}
			}
		}

		// Ensure the winner has a confirmed HandImage.
		// In a normal win (not closing), force 0 if no score was provided.
		if round.WinnerPlayerID != "" {
			existing, _ := database.GetHandImageByRoundAndPlayer(roundID, round.WinnerPlayerID)
			if existing == nil {
				// No image: create with 0 (normal win — winner scores nothing)
				imageID := uuid.New().String()
				if err := database.CreateHandImage(imageID, roundID, round.WinnerPlayerID, "winner:"+imageID); err == nil {
					if err2 := database.UpdateHandImagePoints(imageID, 0, true, "[]"); err2 != nil {
						log.Printf("UpdateHandImagePoints winner error: %v", err2)
					}
				}
			} else if existing.Confirmed == 0 {
				// Image exists but not confirmed: confirm with 0 (normal win)
				if err := database.UpdateHandImagePoints(existing.ID, 0, true, "[]"); err != nil {
					log.Printf("UpdateHandImagePoints winner confirm error: %v", err)
				}
			}
			// If existing.Confirmed == 1: already processed by the loop above (e.g. closing with pts > 0)
		}

		if err := checkRoundComplete(database, hub, matchID, roundID); err != nil {
			log.Printf("checkRoundComplete error: %v", err)
		}

		http.Redirect(w, r, "/match/"+matchID+"/ranking", http.StatusSeeOther)
	}
}

// ManualScoreHandler handles POST /match/{id}/player/{pid}/score — manual score correction.
func ManualScoreHandler(database db.Store, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.PathValue("id")
		playerID := r.PathValue("pid")

		if !IsHost(r, matchID) {
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}
		if !CheckActionRateLimit(r) {
			http.Error(w, "too many requests, please wait", http.StatusTooManyRequests)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		if !ValidateCSRFToken(r.FormValue("_csrf"), matchID) {
			http.Error(w, "invalid security token", http.StatusForbidden)
			return
		}

		// Validate that the player belongs to this match
		player, err := database.GetPlayer(playerID)
		if err != nil || player.MatchID != matchID {
			http.Error(w, "player not found", http.StatusNotFound)
			return
		}

		scoreStr := r.FormValue("score")
		score, err := strconv.Atoi(scoreStr)
		if err != nil || score < 0 || score > maxScore {
			http.Error(w, "invalid score (0–200)", http.StatusBadRequest)
			return
		}

		if err := database.SetPlayerScore(playerID, score); err != nil {
			log.Printf("SetPlayerScore error: %v", err)
			http.Error(w, "failed to save score", http.StatusInternalServerError)
			return
		}

		hub.Broadcast(matchID, "points_updated")
		http.Redirect(w, r, "/match/"+matchID+"/ranking", http.StatusSeeOther)
	}
}

// CancelMatchHandler handles POST /match/{id}/cancel — cancela a partida e desconecta todos.
func CancelMatchHandler(database db.Store, hub *SSEHub) http.HandlerFunc {
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
		if err := r.ParseForm(); err != nil || !ValidateCSRFToken(r.FormValue("_csrf"), matchID) {
			http.Error(w, "invalid security token", http.StatusForbidden)
			return
		}

		if err := database.UpdateMatchStatus(matchID, "cancelled"); err != nil {
			log.Printf("UpdateMatchStatus error: %v", err)
			http.Error(w, "failed to cancel match", http.StatusInternalServerError)
			return
		}

		hub.Broadcast(matchID, "match_cancelled")
		http.Redirect(w, r, "/?msg=cancelled", http.StatusSeeOther)
	}
}

// FinishMatchHandler handles POST /match/{id}/finish — marks match as finished.
func FinishMatchHandler(database db.Store, hub *SSEHub) http.HandlerFunc {
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
		if err := r.ParseForm(); err != nil || !ValidateCSRFToken(r.FormValue("_csrf"), matchID) {
			http.Error(w, "invalid security token", http.StatusForbidden)
			return
		}

		if err := database.UpdateMatchStatus(matchID, "finished"); err != nil {
			log.Printf("UpdateMatchStatus error: %v", err)
			http.Error(w, "failed to finish match", http.StatusInternalServerError)
			return
		}

		hub.Broadcast(matchID, "match_finished")
		http.Redirect(w, r, "/match/"+matchID+"/ranking", http.StatusSeeOther)
	}
}

// BuildTileStats builds statistics for tiles seen in a round.
func BuildTileStats(handImages []models.HandImage, tableJSON string, totalTiles int) models.TileStats {
	seen := map[string]struct{}{}

	var handTiles []string
	for _, hi := range handImages {
		if hi.Confirmed != 1 {
			continue
		}
		tiles := unmarshalTiles(hi.DetectedTilesJSON)
		for _, t := range tiles {
			if _, ok := seen[t]; !ok {
				seen[t] = struct{}{}
				handTiles = append(handTiles, t)
			}
		}
	}

	var tableTiles []string
	for _, t := range unmarshalTiles(tableJSON) {
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			tableTiles = append(tableTiles, t)
		}
	}

	allSeen := make([]string, 0, len(seen))
	for t := range seen {
		allSeen = append(allSeen, t)
	}

	remaining := totalTiles - len(seen)
	if remaining < 0 {
		remaining = 0
	}

	return models.TileStats{
		HandTiles:      handTiles,
		TableTiles:     tableTiles,
		SeenTiles:      allSeen,
		TotalInSet:     totalTiles,
		SeenCount:      len(seen),
		RemainingCount: remaining,
		HasTablePhoto:  tableJSON != "" && tableJSON != "[]",
	}
}

func marshalTiles(tiles []string) string {
	if len(tiles) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(tiles)
	return string(b)
}

func unmarshalTiles(s string) []string {
	if s == "" || s == "[]" {
		return nil
	}
	var tiles []string
	json.Unmarshal([]byte(s), &tiles)
	return tiles
}
