package handler

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/leandrodaf/domino-placar/internal/db"
	"github.com/leandrodaf/domino-placar/internal/game"
)

// ─── Session Manager ──────────────────────────────────────────────────────────

// GameSessionManager caches active game sessions in memory, backed by DB.
type GameSessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*game.GameSession
	store    db.GameStore
}

func NewGameSessionManager(store db.GameStore) *GameSessionManager {
	return &GameSessionManager{
		sessions: make(map[string]*game.GameSession),
		store:    store,
	}
}

func (m *GameSessionManager) Get(id string) *game.GameSession {
	m.mu.RLock()
	gs := m.sessions[id]
	m.mu.RUnlock()
	if gs != nil {
		return gs
	}
	gs, err := m.store.LoadGameSession(id)
	if err != nil {
		return nil
	}
	m.mu.Lock()
	m.sessions[id] = gs
	m.mu.Unlock()
	return gs
}

func (m *GameSessionManager) Set(gs *game.GameSession) {
	m.mu.Lock()
	m.sessions[gs.ID] = gs
	m.mu.Unlock()
}

func (m *GameSessionManager) Persist(gs *game.GameSession) {
	if err := m.store.SaveGameSession(gs); err != nil {
		log.Printf("persist game session %s: %v", gs.ID, err)
	}
	for _, p := range gs.Participants {
		if err := m.store.UpsertGameParticipant(gs.ID, p); err != nil {
			log.Printf("persist participant %s: %v", p.ID, err)
		}
	}
}

// ─── Cookie helpers ──────────────────────────────────────────────────────────

func getGamePlayerID(r *http.Request, sessionID string) string {
	c, err := r.Cookie("domino_gp_" + sessionID)
	if err != nil {
		return ""
	}
	return c.Value
}

func setGamePlayerCookie(w http.ResponseWriter, sessionID, participantID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "domino_gp_" + sessionID,
		Value:    participantID,
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ─── Bot automation ──────────────────────────────────────────────────────────

// scheduleBotMoves triggers bot turns via goroutine after human action.
func scheduleBotMoves(gs *game.GameSession, mgr *GameSessionManager, hub *SSEHub) {
	go func() {
		for {
			cp := gs.CurrentPlayer()
			if cp == nil || !cp.IsBot || gs.Status != game.SessionActive {
				return
			}
			time.Sleep(game.BotThinkDelay())

			cp = gs.CurrentPlayer()
			if cp == nil || !cp.IsBot || gs.Status != game.SessionActive {
				return
			}

			move := game.BotMove(cp.Hand, gs.Board, gs.Rules.HasBoneyard, len(gs.Boneyard))
			channel := "game:" + gs.ID
			moveID := uuid.New().String()

			var result game.RoundEndResult
			var err error

			switch move.Type {
			case game.MovePlay:
				result, err = gs.PlayTile(cp.ID, move.Tile, move.Side)
				if err != nil {
					log.Printf("bot play error: %v", err)
					return
				}
				_ = mgr.store.RecordGameMove(moveID, gs.ID, cp.ID, gs.RoundNumber, move, 0)
				hub.Broadcast(channel, "game_move_played")

			case game.MoveDraw:
				if _, err = gs.DrawTile(cp.ID); err != nil {
					log.Printf("bot draw error: %v", err)
					return
				}
				hub.Broadcast(channel, "game_turn_changed")
				continue

			case game.MovePass:
				result, err = gs.Pass(cp.ID)
				if err != nil {
					log.Printf("bot pass error: %v", err)
					return
				}
				_ = mgr.store.RecordGameMove(moveID, gs.ID, cp.ID, gs.RoundNumber, move, 0)
				hub.Broadcast(channel, "game_turn_changed")
			}

			mgr.Persist(gs)

			if result.Ended {
				hub.Broadcast(channel, "game_round_ended")
				if gs.Status == game.SessionFinished {
					hub.Broadcast(channel, "game_finished")
					return
				}
				time.Sleep(4 * time.Second)
				gs.StartNextRound()
				mgr.Persist(gs)
				hub.Broadcast(channel, "game_started")
				continue
			}

			// If next player is also a bot, loop continues; otherwise stop
			next := gs.CurrentPlayer()
			if next == nil || !next.IsBot {
				hub.Broadcast(channel, "game_turn_changed")
				return
			}
		}
	}()
}

// ─── Create / Lobby / Join / Start ──────────────────────────────────────────

// CreateGamePageHandler handles GET /game/new — redirects to home with the game panel open.
func CreateGamePageHandler(tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/?game=1", http.StatusSeeOther)
	}
}

// CreateGameHandler handles POST /game — creates a new online game session.
func CreateGameHandler(mgr *GameSessionManager, hub *SSEHub, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !CheckActionRateLimit(r) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		variant := r.FormValue("variant")
		if _, ok := game.Variants[variant]; !ok {
			variant = "pontinho"
		}

		maxPoints := defaultGameMaxPoints(variant)
		if mp, err := strconv.Atoi(r.FormValue("max_points")); err == nil && mp >= 10 && mp <= 500 {
			maxPoints = mp
		}

		teamMode := r.FormValue("team_mode") == "1"
		name := r.FormValue("player_name")
		if name == "" {
			name = "Jogador 1"
		}
		uniqueID := r.FormValue("unique_id")
		if uniqueID == "" {
			uniqueID = uuid.New().String()
		}

		sessionID := uuid.New().String()
		gs := game.NewGameSession(sessionID, variant, maxPoints, teamMode, uniqueID)

		host := &game.Participant{
			ID:       uuid.New().String(),
			Name:     name,
			UniqueID: uniqueID,
			Seat:     0,
			Team:     0,
			IsBot:    false,
		}
		if err := gs.AddParticipant(host); err != nil {
			http.Error(w, "failed to create game", http.StatusInternalServerError)
			return
		}

		if err := mgr.store.CreateGameSession(gs); err != nil {
			log.Printf("CreateGameSession: %v", err)
			http.Error(w, "failed to create game", http.StatusInternalServerError)
			return
		}
		if err := mgr.store.UpsertGameParticipant(sessionID, host); err != nil {
			log.Printf("UpsertGameParticipant: %v", err)
		}
		mgr.Set(gs)

		SetHostCookie(w, sessionID)
		setGamePlayerCookie(w, sessionID, host.ID)
		http.Redirect(w, r, "/game/"+sessionID+"/lobby", http.StatusSeeOther)
	}
}

// GameLobbyHandler handles GET /game/{id}/lobby — the waiting room.
func GameLobbyHandler(mgr *GameSessionManager, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		gs := mgr.Get(sessionID)
		if gs == nil {
			http.Error(w, "game not found", http.StatusNotFound)
			return
		}

		if gs.Status == game.SessionActive || gs.Status == game.SessionFinished {
			http.Redirect(w, r, "/game/"+sessionID+"/play", http.StatusSeeOther)
			return
		}

		variantInfo := game.GetVariant(gs.Variant)
		myPID := getGamePlayerID(r, sessionID)

		// Pre-compute a fixed 4-slot seats array (nil = empty)
		seats := make([]*game.Participant, 4)
		for _, p := range gs.Participants {
			if p.Seat >= 0 && p.Seat < 4 {
				seats[p.Seat] = p
			}
		}

		tmpl.Render(w, r, "game_lobby.html", map[string]any{
			"Session":      gs,
			"Participants": gs.Participants,
			"Seats":        seats,
			"IsHost":       IsHost(r, sessionID),
			"MyPID":        myPID,
			"VariantName":  variantInfo.Name,
			"CSRFToken":    GenerateCSRFToken(sessionID),
		})
	}
}

// GameJoinHandler handles POST /game/{id}/join — adds a player to the lobby.
func GameJoinHandler(mgr *GameSessionManager, hub *SSEHub) http.HandlerFunc {
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
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		name := r.FormValue("player_name")
		if name == "" {
			name = "Jogador"
		}
		uniqueID := r.FormValue("unique_id")
		if uniqueID == "" {
			uniqueID = uuid.New().String()
		}

		// Already in game? Just restore cookie
		if existing := gs.FindParticipant(uniqueID); existing != nil {
			setGamePlayerCookie(w, sessionID, existing.ID)
			http.Redirect(w, r, "/game/"+sessionID+"/lobby", http.StatusSeeOther)
			return
		}

		seat := len(gs.Participants)
		p := &game.Participant{
			ID:       uuid.New().String(),
			Name:     name,
			UniqueID: uniqueID,
			Seat:     seat,
			Team:     seat % 2,
			IsBot:    false,
		}
		if err := gs.AddParticipant(p); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		if err := mgr.store.UpsertGameParticipant(sessionID, p); err != nil {
			log.Printf("UpsertGameParticipant: %v", err)
		}

		setGamePlayerCookie(w, sessionID, p.ID)
		hub.Broadcast("game:"+sessionID, "game_joined")
		http.Redirect(w, r, "/game/"+sessionID+"/lobby", http.StatusSeeOther)
	}
}

// GameStartHandler handles POST /game/{id}/start — starts the game.
func GameStartHandler(mgr *GameSessionManager, hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		if !IsHost(r, sessionID) {
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}
		gs := mgr.Get(sessionID)
		if gs == nil {
			http.Error(w, "game not found", http.StatusNotFound)
			return
		}
		if err := r.ParseForm(); err != nil || !ValidateCSRFToken(r.FormValue("_csrf"), sessionID) {
			http.Error(w, "invalid security token", http.StatusForbidden)
			return
		}

		addBots := r.FormValue("add_bots") != "0"
		if err := gs.StartGame(addBots); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		mgr.Persist(gs)
		hub.Broadcast("game:"+sessionID, "game_started")

		if cp := gs.CurrentPlayer(); cp != nil && cp.IsBot {
			scheduleBotMoves(gs, mgr, hub)
		}

		http.Redirect(w, r, "/game/"+sessionID+"/play", http.StatusSeeOther)
	}
}

// GameJoinPageHandler handles GET /game/{id}/join — shows the join form.
func GameJoinPageHandler(mgr *GameSessionManager, tmpl *Templates) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		gs := mgr.Get(sessionID)
		if gs == nil {
			http.Error(w, "game not found", http.StatusNotFound)
			return
		}
		if gs.Status != game.SessionWaiting {
			http.Redirect(w, r, "/game/"+sessionID+"/play", http.StatusSeeOther)
			return
		}
		variantInfo := game.GetVariant(gs.Variant)
		tmpl.Render(w, r, "game_join.html", map[string]any{
			"Session":     gs,
			"VariantName": variantInfo.Name,
		})
	}
}

// defaultGameMaxPoints returns a sensible default max points for a variant.
func defaultGameMaxPoints(variant string) int {
	switch variant {
	case "pontinho":
		return 51
	case "bloqueio":
		return 100
	case "all_fives":
		return 150
	case "com_pedra":
		return 100
	}
	return 51
}

// GameSSEHandler handles GET /game/{id}/events — SSE stream for game events.
func GameSSEHandler(hub *SSEHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("id")
		channel := "game:" + sessionID
		uid := r.URL.Query().Get("uid")

		if uid != "" {
			hub.PresenceJoin(channel, uid)
			defer hub.PresenceLeave(channel, uid)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		ch := hub.Subscribe(channel)
		defer hub.Unsubscribe(channel, ch)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		_, _ = fmt.Fprintf(w, "data: connected\n\n")
		flusher.Flush()

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case event, ok := <-ch:
				if !ok {
					return
				}
				_, _ = fmt.Fprintf(w, "event: update\ndata: %s\n\n", event)
				flusher.Flush()
			case <-ticker.C:
				_, _ = fmt.Fprintf(w, ": heartbeat\n\n")
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}
