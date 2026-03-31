package game

import "math/rand"

// ValidateMove returns true if the tile can be placed on the given side.
func ValidateMove(board BoardState, tile Tile, side string) bool {
	if board.IsEmpty() {
		return true
	}
	var open int
	if side == "left" {
		open = board.LeftOpen
	} else {
		open = board.RightOpen
	}
	return tile.High == open || tile.Low == open
}

// GetPlayableTiles returns all tiles in the hand that can be played.
func GetPlayableTiles(hand Hand, board BoardState) []Tile {
	if board.IsEmpty() {
		return []Tile(hand)
	}
	seen := map[string]bool{}
	var out []Tile
	for _, t := range hand {
		if t.High == board.LeftOpen || t.Low == board.LeftOpen ||
			t.High == board.RightOpen || t.Low == board.RightOpen {
			if !seen[t.String()] {
				out = append(out, t)
				seen[t.String()] = true
			}
		}
	}
	return out
}

// CanPlay returns true if the player can make any move (play or draw).
func CanPlay(hand Hand, board BoardState, hasBoneyard bool, boneyardLen int) bool {
	if len(GetPlayableTiles(hand, board)) > 0 {
		return true
	}
	return hasBoneyard && boneyardLen > 0
}

// IsBlocked returns true if the player has no valid moves and cannot draw.
func IsBlocked(hand Hand, board BoardState, hasBoneyard bool, boneyardLen int) bool {
	return !CanPlay(hand, board, hasBoneyard, boneyardLen)
}

// ApplyMove places a tile on the board, updating LeftOpen/RightOpen.
// orientation can be "h" (horizontal) or "v" (vertical); defaults to "v" for doubles.
// Returns the updated board state.
func ApplyMove(board BoardState, tile Tile, side string, orientation string) BoardState {
	if orientation == "" {
		if tile.IsDouble() {
			orientation = "v"
		} else {
			orientation = "h"
		}
	}
	var placed PlacedTile
	if board.IsEmpty() {
		placed = PlacedTile{Tile: tile, Flipped: false, Orientation: orientation}
		board.Chain = append(board.Chain, placed)
		board.LeftOpen = tile.Low
		board.RightOpen = tile.High
		board.LayoutRotation = [4]int{0, 90, 180, 270}[rand.Intn(4)]
		return board
	}
	if side == "left" {
		if tile.High == board.LeftOpen {
			placed = PlacedTile{Tile: tile, Flipped: false, Orientation: orientation}
			board.LeftOpen = tile.Low
		} else {
			placed = PlacedTile{Tile: tile, Flipped: true, Orientation: orientation}
			board.LeftOpen = tile.High
		}
		board.Chain = append([]PlacedTile{placed}, board.Chain...)
		board.ChainCenter++
	} else {
		if tile.Low == board.RightOpen {
			placed = PlacedTile{Tile: tile, Flipped: false, Orientation: orientation}
			board.RightOpen = tile.High
		} else {
			placed = PlacedTile{Tile: tile, Flipped: true, Orientation: orientation}
			board.RightOpen = tile.Low
		}
		board.Chain = append(board.Chain, placed)
	}
	return board
}

// RoundEndReason describes why a round ended.
type RoundEndReason string

const (
	ReasonEmptyHand RoundEndReason = "empty_hand"
	ReasonBlocked   RoundEndReason = "blocked"
)

// RoundEndResult is the outcome of a completed round.
type RoundEndResult struct {
	Ended         bool
	WinnerID      string
	Reason        RoundEndReason
	PointsAwarded int
	HandPoints    map[string]int // participantID -> hand points at end

	// Snapshotted under the session lock so callers never read gs fields directly.
	SessionFinished bool // true when the whole game ended (gs.Status == SessionFinished)
	RoundNumber     int  // round number at the time of the action
}

// roundPoints rounds raw pip-sum to the nearest multiple of 5 for All-Fives
// scoring, or returns it unchanged for other modes.
func roundPoints(raw int, mode PointMode) int {
	if mode != PointModeAllFives {
		return raw
	}
	// nearest multiple of 5 (round half-up)
	return ((raw + 2) / 5) * 5
}

// handEffectivePoints returns the hand's pip sum, applying the blank-blank bonus
// rule when applicable: if the hand contains exactly [0|0] and blankBlankBonus
// is true, returns 12 instead of 0.
func handEffectivePoints(h Hand, blankBlankBonus bool) int {
	if blankBlankBonus && len(h) == 1 && h[0] == (Tile{High: 0, Low: 0}) {
		return 12
	}
	return h.Points()
}

// CheckRoundEnd checks if the round should end.
// hands maps participantID → Hand. allBlocked is true when all players are blocked.
// pointMode controls how PointsAwarded is calculated.
// blankBlankBonus enables the pontinho rule: a lone 0|0 tile counts as 12 points.
func CheckRoundEnd(hands map[string]Hand, allBlocked bool, pointMode PointMode, blankBlankBonus bool) RoundEndResult {
	// Any player emptied their hand?
	for id, h := range hands {
		if len(h) == 0 {
			raw := 0
			hpts := map[string]int{}
			for pid, ph := range hands {
				p := handEffectivePoints(ph, blankBlankBonus)
				hpts[pid] = p
				if pid != id {
					raw += p
				}
			}
			return RoundEndResult{Ended: true, WinnerID: id, Reason: ReasonEmptyHand, PointsAwarded: roundPoints(raw, pointMode), HandPoints: hpts}
		}
	}
	// All blocked?
	if allBlocked {
		hpts := map[string]int{}
		minID := ""
		minPts := -1
		total := 0
		for id, h := range hands {
			p := handEffectivePoints(h, blankBlankBonus)
			hpts[id] = p
			total += p
			if minPts < 0 || p < minPts {
				minPts = p
				minID = id
			}
		}
		return RoundEndResult{Ended: true, WinnerID: minID, Reason: ReasonBlocked, PointsAwarded: roundPoints(total, pointMode), HandPoints: hpts}
	}
	return RoundEndResult{Ended: false}
}
