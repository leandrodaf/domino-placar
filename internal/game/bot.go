package game

import (
	"math/rand"
	"time"
)

// rng is the package-level random source, seeded once at startup.
// All game randomness (bot moves, delays, tile shuffle) uses this single source
// to avoid the predictability of per-call rand.New(rand.NewSource(time.Now().UnixNano())),
// which can produce identical sequences when called in rapid succession.
var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// BotStrategy defines the AI personality of a bot player.
type BotStrategy string

const (
	// BotStrategyRandom plays any valid tile at random — unpredictable "curinga".
	BotStrategyRandom BotStrategy = "random"
	// BotStrategyGreedy always plays the highest-value tile — "pedreira" bruta.
	BotStrategyGreedy BotStrategy = "greedy"
	// BotStrategySmart uses heuristics: plays doubles first, controls board ends,
	// prefers tiles that leave a lower-sum hand — "calculista".
	BotStrategySmart BotStrategy = "smart"
)

// BotStrategyLabel returns the human-readable Portuguese name for a strategy.
func BotStrategyLabel(s BotStrategy) string {
	switch s {
	case BotStrategyRandom:
		return "Curinga"
	case BotStrategyGreedy:
		return "Pedreira"
	case BotStrategySmart:
		return "Calculista"
	}
	return "Bot"
}

// BotStrategyEmoji returns an emoji for the strategy badge in the UI.
func BotStrategyEmoji(s BotStrategy) string {
	switch s {
	case BotStrategyRandom:
		return "🎲"
	case BotStrategyGreedy:
		return "💪"
	case BotStrategySmart:
		return "🧠"
	}
	return "🤖"
}

// RandomBotStrategy picks a random strategy for bot creation.
func RandomBotStrategy() BotStrategy {
	strategies := []BotStrategy{BotStrategyRandom, BotStrategyGreedy, BotStrategySmart}
	return strategies[rng.Intn(len(strategies))]
}

// BotMove dispatches to the correct strategy implementation for the given participant.
func BotMove(p *Participant, board BoardState, hasBoneyard bool, boneyardLen int) Move {
	switch p.BotStrategy {
	case BotStrategyRandom:
		return randomBotMove(p.Hand, board, hasBoneyard, boneyardLen)
	case BotStrategySmart:
		return smartBotMove(p.Hand, board, hasBoneyard, boneyardLen)
	default: // BotStrategyGreedy
		return greedyBotMove(p.Hand, board, hasBoneyard, boneyardLen)
	}
}

// BotThinkDelay returns a thinking delay scaled to the bot's strategy.
// Smarter bots take longer to "think".
func BotThinkDelay(strategy BotStrategy) time.Duration {
	var ms int
	switch strategy {
	case BotStrategyRandom:
		ms = 400 + rng.Intn(400) // 400–800 ms
	case BotStrategySmart:
		ms = 1200 + rng.Intn(800) // 1200–2000 ms
	default: // greedy
		ms = 700 + rng.Intn(600) // 700–1300 ms
	}
	return time.Duration(ms) * time.Millisecond
}

// ── Strategy implementations ──────────────────────────────────────────────────

// randomBotMove plays any valid tile chosen at random.
func randomBotMove(hand Hand, board BoardState, hasBoneyard bool, boneyardLen int) Move {
	playable := GetPlayableTiles(hand, board)
	if len(playable) == 0 {
		if hasBoneyard && boneyardLen > 0 {
			return Move{Type: MoveDraw}
		}
		return Move{Type: MovePass}
	}
	tile := playable[rng.Intn(len(playable))]
	orient := "h"
	if tile.IsDouble() {
		orient = "v"
	}
	return Move{Type: MovePlay, Tile: tile, Side: chooseSide(board, tile), Orientation: orient}
}

// greedyBotMove plays the highest-value tile to minimize leftover hand points.
func greedyBotMove(hand Hand, board BoardState, hasBoneyard bool, boneyardLen int) Move {
	playable := GetPlayableTiles(hand, board)
	if len(playable) == 0 {
		if hasBoneyard && boneyardLen > 0 {
			return Move{Type: MoveDraw}
		}
		return Move{Type: MovePass}
	}
	best := playable[0]
	for _, t := range playable[1:] {
		if t.Points() > best.Points() {
			best = t
		}
	}
	orient := "h"
	if best.IsDouble() {
		orient = "v"
	}
	return Move{Type: MovePlay, Tile: best, Side: chooseSide(board, best), Orientation: orient}
}

// smartBotMove picks tiles using a multi-factor heuristic:
//  1. Doubles first — they are hardest to play later and risk a full pip being blocked.
//  2. Board-end control — prefer playing on the end where we have more follow-up tiles,
//     keeping control of that pip and leaving opponents fewer safe plays.
//  3. Minimum remaining hand value — reduces bust risk (pontinho) and lowers end-game score.
func smartBotMove(hand Hand, board BoardState, hasBoneyard bool, boneyardLen int) Move {
	playable := GetPlayableTiles(hand, board)
	if len(playable) == 0 {
		if hasBoneyard && boneyardLen > 0 {
			return Move{Type: MoveDraw}
		}
		return Move{Type: MovePass}
	}
	best := playable[0]
	bestScore := scoreTile(best, hand, board)
	for _, t := range playable[1:] {
		if s := scoreTile(t, hand, board); s > bestScore {
			bestScore = s
			best = t
		}
	}
	orient := "h"
	if best.IsDouble() {
		orient = "v"
	}
	return Move{Type: MovePlay, Tile: best, Side: chooseSide(board, best), Orientation: orient}
}

// scoreTile computes a heuristic score for playing a given tile.
// Higher score = better move.
func scoreTile(tile Tile, hand Hand, board BoardState) int {
	score := 0

	// ① Doubles priority: get rid of doubles early
	if tile.IsDouble() {
		score += 80
	}

	// ② Hand sum reduction: lower remaining hand sum → safer from bust and better end-game
	remaining := 0
	for _, t := range hand {
		if t != tile {
			remaining += t.Points()
		}
	}
	// Scale: max hand sum ~7 tiles × 12 pts = 84. Invert so lower is better.
	score += max(0, 84-remaining)

	// ③ Board-end control: prefer the side where we have more follow-up tiles in hand
	if !board.IsEmpty() {
		leftFollowups := 0
		rightFollowups := 0
		for _, t := range hand {
			if t == tile {
				continue
			}
			if t.High == board.LeftOpen || t.Low == board.LeftOpen {
				leftFollowups++
			}
			if t.High == board.RightOpen || t.Low == board.RightOpen {
				rightFollowups++
			}
		}
		tileFitsLeft := tile.High == board.LeftOpen || tile.Low == board.LeftOpen
		tileFitsRight := tile.High == board.RightOpen || tile.Low == board.RightOpen
		if tileFitsLeft && leftFollowups >= rightFollowups {
			score += 15
		}
		if tileFitsRight && rightFollowups > leftFollowups {
			score += 15
		}
	}

	return score
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func chooseSide(board BoardState, tile Tile) string {
	if board.IsEmpty() {
		return "right"
	}
	leftFits := tile.High == board.LeftOpen || tile.Low == board.LeftOpen
	rightFits := tile.High == board.RightOpen || tile.Low == board.RightOpen
	if leftFits && !rightFits {
		return "left"
	}
	if rightFits && !leftFits {
		return "right"
	}
	if rng.Intn(2) == 0 {
		return "left"
	}
	return "right"
}
