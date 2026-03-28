package handler

// TileInfo describes how many tiles each player draws and how many remain for drawing during the game.
type TileInfo struct {
	TilesEach  int
	TotalTiles int
	Reserve    int
	SetType    string // "double-6" or "double-9"
}

// CalcTiles returns the tile distribution per player for Pontinho.
//
// All games use double-6 (28 tiles).
// Distribution: max tiles per player keeping ≥7 in reserve for drawing.
//
//   2 → 9 each, 10 reserve
//   3 → 7 each,  7 reserve
//   4 → 5 each,  8 reserve
//   5 → 4 each,  8 reserve
//   6 → 3 each, 10 reserve
//   7 → 3 each,  7 reserve
//   8 → 2 each, 12 reserve
//   9 → 2 each, 10 reserve
//  10 → 2 each,  8 reserve
func CalcTiles(playerCount int) TileInfo {
	const d6 = 28

	switch playerCount {
	case 2:
		return TileInfo{9, d6, d6 - 9*2, "double-6"}  // 10 to draw
	case 3:
		return TileInfo{7, d6, d6 - 7*3, "double-6"}  //  7 to draw
	case 4:
		return TileInfo{5, d6, d6 - 5*4, "double-6"}  //  8 to draw
	case 5:
		return TileInfo{4, d6, d6 - 4*5, "double-6"}  //  8 to draw
	case 6:
		return TileInfo{3, d6, d6 - 3*6, "double-6"}  // 10 to draw
	case 7:
		return TileInfo{3, d6, d6 - 3*7, "double-6"}  //  7 to draw
	case 8:
		return TileInfo{2, d6, d6 - 2*8, "double-6"}  // 12 to draw
	case 9:
		return TileInfo{2, d6, d6 - 2*9, "double-6"}  // 10 to draw
	case 10:
		return TileInfo{2, d6, d6 - 2*10, "double-6"} //  8 to draw
	default:
		if playerCount <= 0 {
			return TileInfo{0, 0, 0, "—"}
		}
		each := 2
		reserve := d6 - each*playerCount
		if reserve < 0 {
			reserve = 0
		}
		return TileInfo{each, d6, reserve, "double-6"}
	}
}
