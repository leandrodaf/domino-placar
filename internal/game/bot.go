package game

import (
	"math/rand"
	"time"
)

// BotMove computes the best move for a bot player.
// Greedy: play highest-value tile to minimize leftover points.
func BotMove(hand Hand, board BoardState, hasBoneyard bool, boneyardLen int) Move {
	playable := GetPlayableTiles(hand, board)
	if len(playable) == 0 {
		if hasBoneyard && boneyardLen > 0 {
			return Move{Type: MoveDraw}
		}
		return Move{Type: MovePass}
	}

	// Pick highest-value playable tile
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

func chooseSide(board BoardState, tile Tile) string {
	if board.IsEmpty() {
		return "left"
	}
	leftFits := tile.High == board.LeftOpen || tile.Low == board.LeftOpen
	rightFits := tile.High == board.RightOpen || tile.Low == board.RightOpen
	if leftFits && !rightFits {
		return "left"
	}
	if rightFits && !leftFits {
		return "right"
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	if r.Intn(2) == 0 {
		return "left"
	}
	return "right"
}

// BotThinkDelay returns a realistic thinking delay for the bot.
func BotThinkDelay() time.Duration {
	ms := 800 + rand.Intn(700) // 800-1500ms
	return time.Duration(ms) * time.Millisecond
}
