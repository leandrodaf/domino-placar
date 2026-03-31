package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/leandrodaf/domino-placar/internal/game"
)

// GamePlayHandler handles GET /game/{id}/play — the main game board UI.
func GamePlayHandler(mgr *GameSessionManager, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		gs := mgr.Get(sessionID)
		if gs == nil {
			http.Error(w, "game not found", http.StatusNotFound)
			return
		}

		myPID := getGamePlayerID(r, sessionID)
		var myParticipant *game.Participant
		if myPID != "" {
			myParticipant = gs.FindParticipantByID(myPID)
		}
		// Fallback: try uid query param (for direct links)
		if myParticipant == nil {
			if uid := r.URL.Query().Get("uid"); uid != "" {
				myParticipant = gs.FindParticipant(uid)
				if myParticipant != nil {
					setGamePlayerCookie(w, sessionID, myParticipant.ID)
				}
			}
		}

		variantInfo := game.GetVariant(gs.Variant)
		tmpl.Render(w, r, "game_play.html", map[string]any{
			"Session":       gs,
			"MyParticipant": myParticipant,
			"SessionID":     sessionID,
			"VariantName":   variantInfo.Name,
		})
	}
}

// GameStateHandler handles GET /game/{id}/state — returns JSON game state.
func GameStateHandler(mgr *GameSessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		gs := mgr.Get(sessionID)
		if gs == nil {
			http.Error(w, "game not found", http.StatusNotFound)
			return
		}

		// Always derive uniqueID from the signed cookie — never from query params
		// to prevent IDOR (an attacker could pass ?uid=<other_player> to see their hand).
		var uniqueID string
		if pid := getGamePlayerID(r, sessionID); pid != "" {
			if p := gs.FindParticipantByID(pid); p != nil {
				uniqueID = p.UniqueID
			}
		}

		cols, _ := strconv.Atoi(r.URL.Query().Get("cols"))
		if cols < 0 || cols > 4000 {
			cols = 0
		}
		state := gs.StateForPlayer(uniqueID, cols)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(w).Encode(state); err != nil {
			log.Printf("encode state: %v", err)
		}
	}
}

// GameActionHandler handles POST /game/{id}/action — processes a player move.
func GameActionHandler(mgr *GameSessionManager, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		gs := mgr.Get(sessionID)
		if gs == nil {
			http.Error(w, "game not found", http.StatusNotFound)
			return
		}
		if !CheckActionRateLimit(r) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}

		participantID := getGamePlayerID(r, sessionID)
		if participantID == "" {
			http.Error(w, "not in this game", http.StatusForbidden)
			return
		}

		var body struct {
			Type        string `json:"type"`
			Tile        string `json:"tile"`
			Side        string `json:"side"`
			Orientation string `json:"orientation"`
			Cols        int    `json:"cols"`
		}

		// Support both JSON body and form data
		if r.Header.Get("Content-Type") == "application/json" {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid JSON", http.StatusBadRequest)
				return
			}
		} else {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			body.Type = r.FormValue("type")
			body.Tile = r.FormValue("tile")
			body.Side = r.FormValue("side")
			body.Orientation = r.FormValue("orientation")
		}

		if body.Side == "" {
			body.Side = "right"
		}
		if body.Orientation != "" && body.Orientation != "h" && body.Orientation != "v" {
			http.Error(w, "invalid orientation", http.StatusBadRequest)
			return
		}
		if body.Cols < 0 || body.Cols > 4000 {
			body.Cols = 0
		}

		channel := "game:" + sessionID

		var result game.RoundEndResult
		var actionErr error
		move := game.Move{Type: game.MoveType(body.Type)}

		switch body.Type {
		case "play":
			tile, err := parseTile(body.Tile)
			if err != nil {
				http.Error(w, "invalid tile: "+body.Tile, http.StatusBadRequest)
				return
			}
			// Side auto-detection is handled inside PlayTile under the session lock.
			move.Tile = tile
			move.Side = body.Side
			move.Orientation = body.Orientation
			result, actionErr = gs.PlayTile(participantID, tile, body.Side, body.Orientation)
			if actionErr == nil {
				hub.Broadcast(channel, "game_move_played")
			}

		case "pass":
			result, actionErr = gs.Pass(participantID)
			if actionErr == nil {
				hub.Broadcast(channel, "game_turn_changed")
			}

		case "draw":
			drawnTile, err := gs.DrawTile(participantID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			move.Tile = drawnTile
			hub.Broadcast(channel, "game_turn_changed")
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]any{
				"drawn": drawnTile.String(),
				"state": gs.StateForPlayer(getUniqueIDFromPID(gs, participantID), body.Cols),
			}); err != nil {
				log.Printf("encode draw state: %v", err)
			}
			return

		default:
			http.Error(w, "unknown action type: "+body.Type, http.StatusBadRequest)
			return
		}

		if actionErr != nil {
			log.Printf("[action] error pid=%s session=%s type=%s tile=%s: %v", participantID, sessionID, body.Type, body.Tile, actionErr)
			http.Error(w, actionErr.Error(), http.StatusBadRequest)
			return
		}

		// result.RoundNumber and result.SessionFinished are snapshotted under lock inside PlayTile/Pass.
	log.Printf("[action] ok pid=%s session=%s type=%s tile=%s round=%d", participantID, sessionID, body.Type, body.Tile, result.RoundNumber)

		// Record move to DB
		moveID := uuid.New().String()
		_ = mgr.store.RecordGameMove(moveID, sessionID, participantID, result.RoundNumber, move, 0)
		mgr.Persist(gs)

		if result.Ended {
			log.Printf("[round] ended session=%s round=%d finished=%v", sessionID, result.RoundNumber, result.SessionFinished)
			hub.Broadcast(channel, "game_round_ended")
			if result.SessionFinished {
				hub.Broadcast(channel, "game_finished")
			} else {
				// Auto-start next round
				go func() {
					waitAndStartNextRound(gs, mgr, hub, channel)
				}()
			}
		} else {
			// Trigger bot if it's their turn
			if cp := gs.CurrentPlayer(); cp != nil && cp.IsBot {
				scheduleBotMoves(gs, mgr, hub)
			}
		}

		// Return updated state
		uniqueID := getUniqueIDFromPID(gs, participantID)
		state := gs.StateForPlayer(uniqueID, body.Cols)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(state); err != nil {
			log.Printf("encode action state: %v", err)
		}
	}
}

func waitAndStartNextRound(gs *game.GameSession, mgr *GameSessionManager, hub *SSEHub, channel string) {
	// Give players time to see the round result (must match frontend overlay duration).
	time.Sleep(8 * time.Second)
	// A player may have abandoned the session during the sleep; don't start a new round.
	if gs.IsFinished() {
		return
	}
	gs.StartNextRound()
	mgr.Persist(gs)
	hub.Broadcast(channel, "game_started")
	if cp := gs.CurrentPlayer(); cp != nil && cp.IsBot {
		scheduleBotMoves(gs, mgr, hub)
	}
}

// parseTile parses a tile string like "3-6" into a Tile.
// Values must be in [0, 6] (standard double-six domino set).
func parseTile(s string) (game.Tile, error) {
	var h, l int
	if _, err := fmt.Sscanf(s, "%d-%d", &h, &l); err != nil {
		return game.Tile{}, fmt.Errorf("invalid tile format %q: %w", s, err)
	}
	if h < 0 || h > 6 || l < 0 || l > 6 {
		return game.Tile{}, fmt.Errorf("tile values out of range [0,6]: %q", s)
	}
	return game.Tile{High: h, Low: l}, nil
}

// getUniqueIDFromPID finds a participant's unique_id by their participant ID.
func getUniqueIDFromPID(gs *game.GameSession, pid string) string {
	if p := gs.FindParticipantByID(pid); p != nil {
		return p.UniqueID
	}
	return ""
}
