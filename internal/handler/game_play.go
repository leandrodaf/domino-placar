package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
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

		uniqueID := r.URL.Query().Get("uid")
		if uniqueID == "" {
			if pid := getGamePlayerID(r, sessionID); pid != "" {
				if p := gs.FindParticipantByID(pid); p != nil {
					uniqueID = p.UniqueID
				}
			}
		}

		state := gs.StateForPlayer(uniqueID)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(state)
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
			// If board is empty, side doesn't matter
			if gs.Board.IsEmpty() {
				body.Side = "right"
			}
			// Auto-detect side if not given or both work
			if !game.ValidateMove(gs.Board, tile, body.Side) {
				if game.ValidateMove(gs.Board, tile, "left") {
					body.Side = "left"
				} else if game.ValidateMove(gs.Board, tile, "right") {
					body.Side = "right"
				}
			}
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
			_ = json.NewEncoder(w).Encode(map[string]any{
				"drawn": drawnTile.String(),
				"state": gs.StateForPlayer(getUniqueIDFromPID(gs, participantID)),
			})
			return

		default:
			http.Error(w, "unknown action type: "+body.Type, http.StatusBadRequest)
			return
		}

		if actionErr != nil {
			http.Error(w, actionErr.Error(), http.StatusBadRequest)
			return
		}

		// Record move to DB
		moveID := uuid.New().String()
		_ = mgr.store.RecordGameMove(moveID, sessionID, participantID, gs.RoundNumber, move, 0)
		mgr.Persist(gs)

		if result.Ended {
			hub.Broadcast(channel, "game_round_ended")
			if gs.Status == game.SessionFinished {
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
		state := gs.StateForPlayer(uniqueID)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(state)
	}
}

func waitAndStartNextRound(gs *game.GameSession, mgr *GameSessionManager, hub *SSEHub, channel string) {
	// Give players time to see the round result
	time.Sleep(4 * time.Second)
	gs.StartNextRound()
	mgr.Persist(gs)
	hub.Broadcast(channel, "game_started")
	if cp := gs.CurrentPlayer(); cp != nil && cp.IsBot {
		scheduleBotMoves(gs, mgr, hub)
	}
}

// parseTile parses a tile string like "3-6" into a Tile.
func parseTile(s string) (game.Tile, error) {
	var h, l int
	if _, err := fmt.Sscanf(s, "%d-%d", &h, &l); err != nil {
		return game.Tile{}, fmt.Errorf("invalid tile format %q: %w", s, err)
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
