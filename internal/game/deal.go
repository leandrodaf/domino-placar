package game


// playerTableConfig holds the tile distribution for each player count.
// The game always uses the standard Double-6 set (28 tiles, pips 0–6).
// TilesPerPlayer is reduced for larger groups so the total dealt never
// exceeds the 28 available tiles.
type playerTableConfig struct {
	TilesPerPlayer int
	MaxPip         int
}

var playerTable = map[int]playerTableConfig{
	2:  {7, 6}, // 14 tiles dealt, 14 boneyard
	3:  {7, 6}, // 21 tiles dealt,  7 boneyard
	4:  {7, 6}, // 28 tiles dealt,  0 boneyard
	5:  {5, 6}, // 25 tiles dealt,  3 boneyard
	6:  {4, 6}, // 24 tiles dealt,  4 boneyard
	7:  {4, 6}, // 28 tiles dealt,  0 boneyard
	8:  {3, 6}, // 24 tiles dealt,  4 boneyard
	9:  {3, 6}, // 27 tiles dealt,  1 boneyard
	10: {2, 6}, // 20 tiles dealt,  8 boneyard
}

// PlayerTableConfig returns the tile distribution config for n players.
// Falls back to the closest known config if n is out of range.
func PlayerTableConfig(n int) playerTableConfig {
	if cfg, ok := playerTable[n]; ok {
		return cfg
	}
	if n <= 2 {
		return playerTable[2]
	}
	return playerTable[10]
}

// FullDeckForMaxPip generates all unique domino tiles for pips 0..maxPip.
// Double-6 (maxPip=6) → 28 tiles, which is the only set used by this game.
func FullDeckForMaxPip(maxPip int) []Tile {
	var tiles []Tile
	for high := 0; high <= maxPip; high++ {
		for low := 0; low <= high; low++ {
			tiles = append(tiles, Tile{High: high, Low: low})
		}
	}
	return tiles
}

// FullDeck returns the standard Double-6 deck (28 tiles) for backward-compat.
func FullDeck() []Tile { return FullDeckForMaxPip(6) }

// ShuffleTiles returns a shuffled copy of the given tiles.
// Uses the package-level rng (defined in bot.go) to avoid predictable sequences
// from per-call seeding.
func ShuffleTiles(tiles []Tile) []Tile {
	out := make([]Tile, len(tiles))
	copy(out, tiles)
	rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// Deal distributes tiles to n players using the appropriate deck and tile count
// derived from the player count. Returns (hands, boneyard).
func Deal(n, _ int) ([]Hand, []Tile) {
	cfg := PlayerTableConfig(n)
	deck := ShuffleTiles(FullDeckForMaxPip(cfg.MaxPip))
	tpp := cfg.TilesPerPlayer
	hands := make([]Hand, n)
	for i := 0; i < n; i++ {
		start := i * tpp
		end := start + tpp
		if end > len(deck) {
			end = len(deck)
		}
		hands[i] = Hand(deck[start:end])
	}
	boneyard := deck[n*tpp:]
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
