package game

import (
	"math/rand"
	"time"
)

// FullDeck returns all 28 standard domino tiles.
func FullDeck() []Tile {
	var tiles []Tile
	for high := 0; high <= 6; high++ {
		for low := 0; low <= high; low++ {
			tiles = append(tiles, Tile{High: high, Low: low})
		}
	}
	return tiles
}

// ShuffleTiles returns a shuffled copy of the given tiles.
func ShuffleTiles(tiles []Tile) []Tile {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	out := make([]Tile, len(tiles))
	copy(out, tiles)
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// Deal distributes tiles to n players, returns (hands, boneyard).
func Deal(n, tilesPerPlayer int) ([]Hand, []Tile) {
	deck := ShuffleTiles(FullDeck())
	hands := make([]Hand, n)
	for i := 0; i < n; i++ {
		start := i * tilesPerPlayer
		end := start + tilesPerPlayer
		if end > len(deck) {
			end = len(deck)
		}
		hands[i] = Hand(deck[start:end])
	}
	boneyard := deck[n*tilesPerPlayer:]
	return hands, boneyard
}

// FindFirstPlayer returns the seat index of the player who holds [6|6],
// or the highest double, or seat 0 if no doubles.
func FindFirstPlayer(hands []Hand) int {
	bestSeat := 0
	bestVal := -1
	for i, hand := range hands {
		for _, t := range hand {
			if t.IsDouble() && t.High > bestVal {
				bestVal = t.High
				bestSeat = i
			}
		}
	}
	return bestSeat
}
