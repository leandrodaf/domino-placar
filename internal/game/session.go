package game

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"
)

// SessionStatus is the lifecycle state of a game session.
type SessionStatus string

const (
	SessionWaiting  SessionStatus = "waiting"
	SessionActive   SessionStatus = "active"
	SessionFinished SessionStatus = "finished"
)

// Participant is a player in the game (human or bot).
type Participant struct {
	ID          string
	Name        string
	UniqueID    string
	Seat        int
	Team        int // 0 or 1
	IsBot       bool
	BotStrategy BotStrategy // only meaningful when IsBot=true
	TotalScore  int
	Eliminated  bool // true once TotalScore >= MaxPoints in losers_pay mode
	Hand        Hand
}

// GameSession holds the full in-memory state of one game.
type GameSession struct {
	mu sync.Mutex

	ID         string
	Variant    string
	Rules      VariantRules
	MaxPoints  int
	TeamMode   bool
	QuickPlay       bool // true when created via quickplay matchmaking
	PendingBotCount int
	Status          SessionStatus
	HostID     string // UniqueID of the host

	Participants []*Participant // ordered by seat
	TurnIdx      int            // current turn index into Participants

	// Round state
	RoundNumber   int
	Board         BoardState
	Boneyard      []Tile
	PassCount     int             // consecutive passes in current round
	BlockedIDs    map[string]bool
	LastRound     *RoundEndResult // result of the most recently completed round

	CreatedAt time.Time
}

// NewGameSession creates a fresh session.
func NewGameSession(id, variant string, maxPoints int, teamMode bool, hostUID string) *GameSession {
	return &GameSession{
		ID:         id,
		Variant:    variant,
		Rules:      GetVariant(variant),
		MaxPoints:  maxPoints,
		TeamMode:   teamMode,
		Status:     SessionWaiting,
		HostID:     hostUID,
		BlockedIDs: map[string]bool{},
		CreatedAt:  time.Now(),
	}
}

// AddParticipant adds a player to the lobby. Returns error if full or already joined.
func (gs *GameSession) AddParticipant(p *Participant) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	if gs.Status != SessionWaiting {
		return fmt.Errorf("game already started")
	}
	if len(gs.Participants) >= 10 {
		return fmt.Errorf("game is full")
	}
	for _, existing := range gs.Participants {
		if existing.UniqueID == p.UniqueID {
			return fmt.Errorf("already in game")
		}
	}
	p.Seat = len(gs.Participants)
	gs.Participants = append(gs.Participants, p)
	return nil
}

// RemoveParticipant removes a participant by UniqueID from a waiting session.
// Returns (removed, remaining): removed is false if not found or game is not waiting;
// remaining is the participant count after removal (valid only when removed is true).
func (gs *GameSession) RemoveParticipant(uniqueID string) (bool, int) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	if gs.Status != SessionWaiting {
		return false, len(gs.Participants)
	}
	idx := -1
	for i, p := range gs.Participants {
		if p.UniqueID == uniqueID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false, len(gs.Participants)
	}
	gs.Participants = append(gs.Participants[:idx], gs.Participants[idx+1:]...)
	// Re-assign seats so they remain contiguous.
	for i, p := range gs.Participants {
		p.Seat = i
		p.Team = i % 2
	}
	// If the departing player was the host, elect the first remaining participant.
	if gs.HostID == uniqueID && len(gs.Participants) > 0 {
		gs.HostID = gs.Participants[0].UniqueID
	}
	return true, len(gs.Participants)
}

// Lock acquires the session mutex for external callers that need atomic
// multi-field access (e.g., persistence snapshots). Prefer the existing
// public methods over direct lock usage whenever possible.
func (gs *GameSession) Lock() { gs.mu.Lock() }

// Unlock releases the session mutex.
func (gs *GameSession) Unlock() { gs.mu.Unlock() }

// IsFinished reports whether the session has finished, under the lock.
func (gs *GameSession) IsFinished() bool {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	return gs.Status == SessionFinished
}

// IsAbandoned reports whether a waiting session is old enough to be evicted.
func (gs *GameSession) IsAbandoned() bool {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	return gs.Status == SessionWaiting && time.Since(gs.CreatedAt) > 2*time.Hour
}

// FindParticipant returns a participant by UniqueID, or nil.
func (gs *GameSession) FindParticipant(uniqueID string) *Participant {
	for _, p := range gs.Participants {
		if p.UniqueID == uniqueID {
			return p
		}
	}
	return nil
}

// FindParticipantByID returns a participant by ID, or nil.
func (gs *GameSession) FindParticipantByID(id string) *Participant {
	for _, p := range gs.Participants {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// StartGame deals tiles and begins the first round.
// botCount specifies how many bot players to add (0 = no bots).
// The total number of players (humans + bots) must be between 2 and 10.
func (gs *GameSession) StartGame(botCount int) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	if gs.Status != SessionWaiting {
		return fmt.Errorf("already started")
	}
	if len(gs.Participants) < 1 {
		return fmt.Errorf("need at least 1 player")
	}
	// Clamp botCount so total stays within [2, 10]
	maxBots := 10 - len(gs.Participants)
	if botCount > maxBots {
		botCount = maxBots
	}
	if botCount < 0 {
		botCount = 0
	}
	if len(gs.Participants)+botCount < 2 {
		return fmt.Errorf("need at least 2 players total (add bots or wait for players)")
	}
	for i := 0; i < botCount; i++ {
		seat := len(gs.Participants)
		strategy := RandomBotStrategy()
		gs.Participants = append(gs.Participants, &Participant{
			ID:          fmt.Sprintf("bot-%s-%d", gs.ID, seat),
			Name:        fmt.Sprintf("Bot %s %d", BotStrategyEmoji(strategy), seat),
			UniqueID:    fmt.Sprintf("bot-%s-%d", gs.ID, seat),
			Seat:        seat,
			Team:        seat % 2,
			IsBot:       true,
			BotStrategy: strategy,
		})
	}
	if len(gs.Participants) < 2 {
		return fmt.Errorf("need at least 2 players to start")
	}
	gs.Status = SessionActive
	gs.RoundNumber = 1
	gs.dealRound()
	return nil
}

// dealRound distributes tiles and sets the starting player.
// Round 1: player with the highest double starts.
// Round 2+: winner of the previous round starts.
// Eliminated players receive no tiles.
func (gs *GameSession) dealRound() {
	active := gs.activePlayers()
	n := len(active)
	if n == 0 {
		return // all players eliminated; game should already be finished
	}
	hands, boneyard := Deal(n, gs.Rules.TilesPerPlayer)
	for i, p := range active {
		p.Hand = hands[i]
	}
	// Clear hands for eliminated players.
	for _, p := range gs.Participants {
		if p.Eliminated {
			p.Hand = Hand{}
		}
	}
	gs.Boneyard = boneyard
	gs.Board = BoardState{}
	gs.PassCount = 0
	gs.BlockedIDs = map[string]bool{}

	if gs.RoundNumber == 1 {
		handSlice := make([]Hand, n)
		for i, p := range active {
			handSlice[i] = p.Hand
		}
		firstIdx := FindFirstPlayer(handSlice)
		gs.TurnIdx = active[firstIdx].Seat
	} else if gs.LastRound != nil {
		// Winner of the previous round starts.
		for i, p := range gs.Participants {
			if p.ID == gs.LastRound.WinnerID && !p.Eliminated {
				gs.TurnIdx = i
				return
			}
		}
		// Fallback: first active player.
		if len(active) > 0 {
			gs.TurnIdx = active[0].Seat
		}
	}
}

// CurrentPlayer returns the participant whose turn it is.
func (gs *GameSession) CurrentPlayer() *Participant {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	if len(gs.Participants) == 0 {
		return nil
	}
	return gs.Participants[gs.TurnIdx%len(gs.Participants)]
}

// PlayTile executes a play move. Returns RoundEndResult.
func (gs *GameSession) PlayTile(participantID string, tile Tile, side string, orientation string) (RoundEndResult, error) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	p := gs.FindParticipantByID(participantID)
	if p == nil {
		return RoundEndResult{}, fmt.Errorf("participant not found")
	}
	if gs.Participants[gs.TurnIdx%len(gs.Participants)].ID != participantID {
		return RoundEndResult{}, fmt.Errorf("not your turn")
	}
	if !p.Hand.Contains(tile) {
		return RoundEndResult{}, fmt.Errorf("tile not in hand")
	}
	// Auto-detect side under lock (no external caller needs to pre-check board state).
	if gs.Board.IsEmpty() {
		side = "right"
	} else if !ValidateMove(gs.Board, tile, side) {
		if ValidateMove(gs.Board, tile, "left") {
			side = "left"
		} else if ValidateMove(gs.Board, tile, "right") {
			side = "right"
		} else {
			return RoundEndResult{}, fmt.Errorf("invalid move: tile %s does not fit on either side", tile)
		}
	}

	gs.Board = ApplyMove(gs.Board, tile, side, orientation)
	p.Hand = p.Hand.Remove(tile)
	gs.PassCount = 0
	delete(gs.BlockedIDs, participantID)

	// Check round end
	hands := gs.allHands()
	result := CheckRoundEnd(hands, false, gs.Rules.PointMode, gs.Rules.BlankBlankBonus)
	result.RoundNumber = gs.RoundNumber
	if result.Ended {
		gs.applyRoundResult(result)
		result.SessionFinished = gs.Status == SessionFinished
		return result, nil
	}

	gs.advanceTurn()
	return result, nil
}

// Pass marks a player as passing (only when truly blocked).
func (gs *GameSession) Pass(participantID string) (RoundEndResult, error) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	p := gs.FindParticipantByID(participantID)
	if p == nil {
		return RoundEndResult{}, fmt.Errorf("participant not found")
	}
	if gs.Participants[gs.TurnIdx%len(gs.Participants)].ID != participantID {
		return RoundEndResult{}, fmt.Errorf("not your turn")
	}
	if !IsBlocked(p.Hand, gs.Board, gs.Rules.HasBoneyard, len(gs.Boneyard)) {
		return RoundEndResult{}, fmt.Errorf("cannot pass: you have playable tiles or can draw")
	}

	gs.BlockedIDs[participantID] = true
	gs.PassCount++

	// Check if all active players are blocked.
	allBlocked := len(gs.BlockedIDs) >= len(gs.activePlayers())
	hands := gs.allHands()
	result := CheckRoundEnd(hands, allBlocked, gs.Rules.PointMode, gs.Rules.BlankBlankBonus)
	result.RoundNumber = gs.RoundNumber
	if result.Ended {
		gs.applyRoundResult(result)
		result.SessionFinished = gs.Status == SessionFinished
		return result, nil
	}

	gs.advanceTurn()
	return result, nil
}

// DrawTile draws from boneyard (for variants with boneyard).
func (gs *GameSession) DrawTile(participantID string) (Tile, error) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	p := gs.FindParticipantByID(participantID)
	if p == nil {
		return Tile{}, fmt.Errorf("participant not found")
	}
	if gs.Participants[gs.TurnIdx%len(gs.Participants)].ID != participantID {
		return Tile{}, fmt.Errorf("not your turn")
	}
	if !gs.Rules.HasBoneyard || len(gs.Boneyard) == 0 {
		return Tile{}, fmt.Errorf("no tiles to draw")
	}

	tile := gs.Boneyard[0]
	gs.Boneyard = gs.Boneyard[1:]
	p.Hand = append(p.Hand, tile)
	return tile, nil
}

// ExecuteBotTurn computes and executes a bot move atomically under the session
// lock. This prevents the race where the goroutine reads stale hand/board data
// before calling PlayTile/Pass/DrawTile separately.
//
// Returns the move chosen, the round result, and any execution error.
// If the current player is not a bot or the session is not active, it returns
// an error so the caller can stop the bot loop.
func (gs *GameSession) ExecuteBotTurn() (Move, RoundEndResult, error) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if gs.Status != SessionActive || len(gs.Participants) == 0 {
		return Move{}, RoundEndResult{}, fmt.Errorf("session not active")
	}

	cp := gs.Participants[gs.TurnIdx%len(gs.Participants)]
	if !cp.IsBot {
		return Move{}, RoundEndResult{}, fmt.Errorf("not a bot turn")
	}

	// Compute the best move using the current (locked) state.
	move := BotMove(cp, gs.Board, gs.Rules.HasBoneyard, len(gs.Boneyard))

	switch move.Type {
	case MovePlay:
		if !cp.Hand.Contains(move.Tile) {
			return move, RoundEndResult{}, fmt.Errorf("bot tile not in hand")
		}
		if !ValidateMove(gs.Board, move.Tile, move.Side) {
			return move, RoundEndResult{}, fmt.Errorf("bot move invalid: %s on %s", move.Tile, move.Side)
		}
		gs.Board = ApplyMove(gs.Board, move.Tile, move.Side, move.Orientation)
		cp.Hand = cp.Hand.Remove(move.Tile)
		gs.PassCount = 0
		delete(gs.BlockedIDs, cp.ID)

		hands := gs.allHands()
		result := CheckRoundEnd(hands, false, gs.Rules.PointMode, gs.Rules.BlankBlankBonus)
		result.RoundNumber = gs.RoundNumber
		if result.Ended {
			gs.applyRoundResult(result)
			result.SessionFinished = gs.Status == SessionFinished
			return move, result, nil
		}
		gs.advanceTurn()
		return move, result, nil

	case MoveDraw:
		if !gs.Rules.HasBoneyard || len(gs.Boneyard) == 0 {
			return move, RoundEndResult{}, fmt.Errorf("no tiles to draw")
		}
		tile := gs.Boneyard[0]
		gs.Boneyard = gs.Boneyard[1:]
		cp.Hand = append(cp.Hand, tile)
		// Turn stays with the bot so it can play after drawing.
		return move, RoundEndResult{RoundNumber: gs.RoundNumber}, nil

	case MovePass:
		if !IsBlocked(cp.Hand, gs.Board, gs.Rules.HasBoneyard, len(gs.Boneyard)) {
			return move, RoundEndResult{}, fmt.Errorf("bot cannot pass: has playable tiles")
		}
		gs.BlockedIDs[cp.ID] = true
		gs.PassCount++
		allBlocked := len(gs.BlockedIDs) >= len(gs.activePlayers())
		hands := gs.allHands()
		result := CheckRoundEnd(hands, allBlocked, gs.Rules.PointMode, gs.Rules.BlankBlankBonus)
		result.RoundNumber = gs.RoundNumber
		if result.Ended {
			gs.applyRoundResult(result)
			result.SessionFinished = gs.Status == SessionFinished
			return move, result, nil
		}
		gs.advanceTurn()
		return move, result, nil

	default:
		return move, RoundEndResult{}, fmt.Errorf("unknown move type: %s", move.Type)
	}
}

// StartNextRound begins a new round after a round has ended.
// No-ops if the session is no longer active (e.g. abandoned between the round-end
// broadcast and the 8-second delay before the next deal).
func (gs *GameSession) StartNextRound() {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	if gs.Status != SessionActive {
		return
	}
	gs.RoundNumber++
	gs.dealRound()
}

// CheckGameOver returns true when the game has a winner.
// In losers_pay mode (Pontinho) the last non-eliminated player wins.
// In other modes the first player to reach MaxPoints wins.
func (gs *GameSession) CheckGameOver() (bool, *Participant) {
	if gs.Rules.PointMode == PointModeLosersPay {
		var lastActive *Participant
		active := 0
		for _, p := range gs.Participants {
			if !p.Eliminated {
				active++
				lastActive = p
			}
		}
		if active <= 1 {
			return true, lastActive
		}
		return false, nil
	}
	for _, p := range gs.Participants {
		if p.TotalScore > gs.MaxPoints {
			return true, p
		}
	}
	return false, nil
}

// StateForPlayer returns a JSON-serializable snapshot of the game state
// from the perspective of the given uniqueID (hides other players' hands).

func (gs *GameSession) StateForPlayer(uniqueID string, cols int) map[string]any {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	var myHand Hand
	players := make([]map[string]any, 0, len(gs.Participants))
	currentPlayerID := ""
	if len(gs.Participants) > 0 {
		currentPlayerID = gs.Participants[gs.TurnIdx%len(gs.Participants)].ID
	}

	for _, p := range gs.Participants {
		entry := map[string]any{
			"id":          p.ID,
			"name":        p.Name,
			"unique_id":   p.UniqueID,
			"seat":        p.Seat,
			"team":        p.Team,
			"is_bot":      p.IsBot,
			"total_score": p.TotalScore,
			"eliminated":  p.Eliminated,
			"hand_count":  len(p.Hand),
			"is_turn":     p.ID == currentPlayerID,
		}
		if p.UniqueID == uniqueID {
			myHand = p.Hand
			entry["hand"] = handToStrings(p.Hand)
		}
		players = append(players, entry)
	}

	// Compute pixel positions for the tile chain using the layout engine.
	// Tile dimensions scale with viewport width for good visual proportion.
	boardW := float64(cols)
	if boardW < 200 {
		boardW = 600
	}

	tileLen := math.Round(boardW / 7.0)
	if tileLen < 45 {
		tileLen = 45
	}
	if tileLen > 90 {
		tileLen = 90
	}
	tileWid := math.Round(tileLen / 2.0)
	padding := math.Round(tileWid * 0.4)

	boardH := boardW * 1.5
	cfg := BoardConfig{
		ScreenWidth:  boardW,
		ScreenHeight: boardH,
		TileLength:   tileLen,
		TileWidth:    tileWid,
		Padding:      padding,
	}

	var rendered []RenderedTile
	if len(gs.Board.Chain) > 0 {
		engineCfg := cfg
		if gs.Board.LayoutRotation == 90 || gs.Board.LayoutRotation == 270 {
			engineCfg.ScreenWidth, engineCfg.ScreenHeight = cfg.ScreenHeight, cfg.ScreenWidth
		}

		rendered = RenderBidirectionalChain(engineCfg, gs.Board.Chain, gs.Board.ChainCenter)

		if gs.Board.LayoutRotation != 0 {
			RotateLayout(rendered, engineCfg, cfg, gs.Board.LayoutRotation)
		}
	}

	boardTiles := make([]map[string]any, 0, len(gs.Board.Chain))
	for i, pt := range gs.Board.Chain {
		entry := map[string]any{
			"tile":    pt.Tile.String(),
			"high":    pt.Tile.High,
			"low":     pt.Tile.Low,
			"flipped": pt.Flipped,
		}
		if i < len(rendered) {
			entry["x"] = rendered[i].X
			entry["y"] = rendered[i].Y
			entry["rotation"] = rendered[i].Rotation
		}
		boardTiles = append(boardTiles, entry)
	}

	var playable []string
	if uniqueID != "" {
		pl := gs.FindParticipant(uniqueID)
		if pl != nil && currentPlayerID == pl.ID {
			for _, t := range GetPlayableTiles(pl.Hand, gs.Board) {
				playable = append(playable, t.String())
			}
		}
	}

	result := map[string]any{
		"id":             gs.ID,
		"variant":        gs.Variant,
		"max_points":     gs.MaxPoints,
		"team_mode":      gs.TeamMode,
		"status":         string(gs.Status),
		"round_number":   gs.RoundNumber,
		"current_player": currentPlayerID,
		"board": map[string]any{
			"tiles":        boardTiles,
			"left_open":    gs.Board.LeftOpen,
			"right_open":   gs.Board.RightOpen,
			"tile_count":   len(gs.Board.Chain),
			"canvas_width":  cfg.ScreenWidth,
			"canvas_height": cfg.ScreenHeight,
			"tile_length":   cfg.TileLength,
			"tile_width":    cfg.TileWidth,
		},
		"boneyard_count": len(gs.Boneyard),
		"players":        players,
		"my_hand":        handToStrings(myHand),
		"playable_tiles": playable,
	}
	if gs.LastRound != nil {
		winnerName := ""
		if w := gs.FindParticipantByID(gs.LastRound.WinnerID); w != nil {
			winnerName = w.Name
		}
		lastRound := map[string]any{
			"winner_id":      gs.LastRound.WinnerID,
			"winner_name":    winnerName,
			"points_awarded": gs.LastRound.PointsAwarded,
			"reason":         string(gs.LastRound.Reason),
		}
		// Expose per-player hand points so the UI can show each player's penalty.
		if len(gs.LastRound.HandPoints) > 0 {
			lastRound["hand_points"] = gs.LastRound.HandPoints
		}
		result["last_round"] = lastRound
	}
	return result
}

// activePlayers returns participants who are not eliminated, in seat order.
func (gs *GameSession) activePlayers() []*Participant {
	var out []*Participant
	for _, p := range gs.Participants {
		if !p.Eliminated {
			out = append(out, p)
		}
	}
	return out
}

func (gs *GameSession) allHands() map[string]Hand {
	m := map[string]Hand{}
	for _, p := range gs.Participants {
		if !p.Eliminated {
			m[p.ID] = p.Hand
		}
	}
	return m
}

func (gs *GameSession) advanceTurn() {
	n := len(gs.Participants)
	if n == 0 {
		return
	}
	gs.TurnIdx = (gs.TurnIdx + 1) % n
	// Skip eliminated players — cap at n iterations to prevent infinite loop
	// if all participants are somehow eliminated.
	for i := 0; i < n; i++ {
		if !gs.Participants[gs.TurnIdx%n].Eliminated {
			return
		}
		gs.TurnIdx = (gs.TurnIdx + 1) % n
	}
}

func (gs *GameSession) applyRoundResult(result RoundEndResult) {
	if gs.Rules.PointMode == PointModeLosersPay {
		// Each non-winner receives their own hand value as a penalty.
		for _, p := range gs.Participants {
			if p.Eliminated || p.ID == result.WinnerID {
				continue
			}
			if pts, ok := result.HandPoints[p.ID]; ok {
				p.TotalScore += pts
			}
			if p.TotalScore > gs.MaxPoints {
				p.Eliminated = true
			}
		}
	} else {
		if winner := gs.FindParticipantByID(result.WinnerID); winner != nil {
			winner.TotalScore += result.PointsAwarded
		}
	}
	gs.LastRound = &result
	if over, _ := gs.CheckGameOver(); over {
		gs.Status = SessionFinished
	}
}

func handToStrings(h Hand) []string {
	if h == nil {
		return []string{}
	}
	s := make([]string, len(h))
	for i, t := range h {
		s[i] = t.String()
	}
	return s
}

// BoardToJSON serializes the board for storage.
func BoardToJSON(board BoardState) string {
	data, _ := json.Marshal(board)
	return string(data)
}

// BoardFromJSON deserializes a board.
func BoardFromJSON(s string) BoardState {
	var b BoardState
	if s != "" {
		_ = json.Unmarshal([]byte(s), &b)
	}
	return b
}

// BoneyardToJSON serializes the boneyard for storage.
func BoneyardToJSON(tiles []Tile) string {
	data, _ := json.Marshal(tiles)
	return string(data)
}

// BoneyardFromJSON deserializes a boneyard.
func BoneyardFromJSON(s string) []Tile {
	var tiles []Tile
	if s != "" {
		_ = json.Unmarshal([]byte(s), &tiles)
	}
	return tiles
}

// HandToJSON serializes a hand.
func HandToJSON(h Hand) string {
	data, _ := json.Marshal(h)
	return string(data)
}

// HandFromJSON deserializes a hand.
func HandFromJSON(s string) Hand {
	var h Hand
	if s != "" {
		_ = json.Unmarshal([]byte(s), &h)
	}
	return h
}
