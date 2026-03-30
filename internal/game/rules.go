package game

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
}

// CheckRoundEnd checks if the round should end.
// hands maps participantID → Hand. allBlocked is true when all players are blocked.
func CheckRoundEnd(hands map[string]Hand, allBlocked bool) RoundEndResult {
	// Any player emptied their hand?
	for id, h := range hands {
		if len(h) == 0 {
			sum := 0
			hpts := map[string]int{}
			for pid, ph := range hands {
				p := ph.Points()
				hpts[pid] = p
				if pid != id {
					sum += p
				}
			}
			return RoundEndResult{Ended: true, WinnerID: id, Reason: ReasonEmptyHand, PointsAwarded: sum, HandPoints: hpts}
		}
	}
	// All blocked?
	if allBlocked {
		hpts := map[string]int{}
		minID := ""
		minPts := -1
		total := 0
		for id, h := range hands {
			p := h.Points()
			hpts[id] = p
			total += p
			if minPts < 0 || p < minPts {
				minPts = p
				minID = id
			}
		}
		return RoundEndResult{Ended: true, WinnerID: minID, Reason: ReasonBlocked, PointsAwarded: total, HandPoints: hpts}
	}
	return RoundEndResult{Ended: false}
}
