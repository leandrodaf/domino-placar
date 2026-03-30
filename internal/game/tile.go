package game

import "fmt"

// Tile represents a domino piece.
type Tile struct {
	High int
	Low  int
}

func (t Tile) String() string { return fmt.Sprintf("%d-%d", t.High, t.Low) }
func (t Tile) Points() int    { return t.High + t.Low }
func (t Tile) IsDouble() bool { return t.High == t.Low }

// PlacedTile is a tile on the board with display orientation.
type PlacedTile struct {
	Tile    Tile
	Flipped bool // display Low on top
}

// BoardState is the current board.
type BoardState struct {
	Chain     []PlacedTile
	LeftOpen  int
	RightOpen int
}

func (b BoardState) IsEmpty() bool { return len(b.Chain) == 0 }

// Hand is a player's tiles.
type Hand []Tile

func (h Hand) Points() int {
	t := 0
	for _, tile := range h {
		t += tile.Points()
	}
	return t
}

func (h Hand) Remove(t Tile) Hand {
	result := make(Hand, 0, len(h))
	removed := false
	for _, tile := range h {
		if !removed && tile == t {
			removed = true
			continue
		}
		result = append(result, tile)
	}
	return result
}

func (h Hand) Contains(t Tile) bool {
	for _, tile := range h {
		if tile == t {
			return true
		}
	}
	return false
}

// MoveType is the kind of action a player takes.
type MoveType string

const (
	MovePlay MoveType = "play"
	MovePass MoveType = "pass"
	MoveDraw MoveType = "draw"
)

// Move is a player action.
type Move struct {
	Type MoveType
	Tile Tile   // only for MovePlay
	Side string // "left" or "right"
}
