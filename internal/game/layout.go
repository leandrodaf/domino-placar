package game

// ──────────────────────────────────────────────────────────────────────────────
// Snake Layout — server-side board positioning engine.
//
// Calculates (X, Y) grid coordinates for domino tiles so they "serpentine"
// within the canvas boundaries instead of leaking off-screen.
// ──────────────────────────────────────────────────────────────────────────────

// Direction represents the flow direction of tile placement.
type Direction int

const (
	East  Direction = iota // left → right
	West                   // right → left
	South                  // top → bottom
	North                  // bottom → top
)

// String returns a human-readable direction name.
func (d Direction) String() string {
	switch d {
	case East:
		return "East"
	case West:
		return "West"
	case South:
		return "South"
	case North:
		return "North"
	default:
		return "?"
	}
}

// Opposite returns the reverse direction.
func (d Direction) Opposite() Direction {
	switch d {
	case East:
		return West
	case West:
		return East
	case South:
		return North
	case North:
		return South
	default:
		return d
	}
}

// IsHorizontal reports whether the direction flows along the X axis.
func (d Direction) IsHorizontal() bool { return d == East || d == West }

// LayoutTile holds the computed position and orientation of a tile on the grid.
type LayoutTile struct {
	Tile      Tile
	X         int       // grid column (0-based)
	Y         int       // grid row (0-based)
	Direction Direction // flow direction when this tile was placed
	IsDouble  bool      // true for buchas/carroças (rendered vertically)
	IsCurve   bool      // true if this tile was a forced curve piece
}

// BoardLayout holds the full grid state and the chain of placed tiles.
type BoardLayout struct {
	// Grid dimensions (in cells).
	MaxCols int
	MaxRows int

	// Padding (in cells) — the algorithm forces a curve before reaching
	// this distance from any border.
	Padding int

	// The ordered chain of tiles placed on the board.
	Tiles []LayoutTile

	// Occupied tracks which cells are taken: occupied[y][x] = true.
	Occupied [][]bool
}

// NewBoardLayout creates a layout for a canvas of cols × rows cells.
// padding is the number of cells to keep free before the edges.
func NewBoardLayout(cols, rows, padding int) *BoardLayout {
	occ := make([][]bool, rows)
	for r := range occ {
		occ[r] = make([]bool, cols)
	}
	return &BoardLayout{
		MaxCols:  cols,
		MaxRows:  rows,
		Padding:  padding,
		Tiles:    nil,
		Occupied: occ,
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Cell sizing: a normal tile occupies 2 horizontal cells (pontaA + pontaB),
// while a double (bucha) occupies 1 column × 2 rows when rendered vertically.
// ──────────────────────────────────────────────────────────────────────────────

const (
	normalLenH = 2 // horizontal cells for a normal tile laid horizontally
	normalLenV = 1 // horizontal cells when going vertical (transition piece)
	doubleLenH = 1 // horizontal cells for a double laid horizontally
	doubleLenV = 1 // horizontal cells for a double rendered vertically
)

// tileWidth returns how many columns the tile occupies given the flow direction.
func tileWidth(isDouble bool, dir Direction) int {
	if dir.IsHorizontal() {
		if isDouble {
			return doubleLenH
		}
		return normalLenH
	}
	// Vertical flow: tile rendered sideways — takes 1 column.
	return 1
}

// tileHeight returns how many rows the tile occupies given the flow direction.
func tileHeight(isDouble bool, dir Direction) int {
	if dir.IsHorizontal() {
		if isDouble {
			return 2 // double rendered vertically even in horizontal flow
		}
		return 1
	}
	// Vertical flow.
	if isDouble {
		return 1 // double in vertical flow: rendered horizontally, 1 row
	}
	return 2 // normal tile rotated into vertical occupies 2 rows
}

// ──────────────────────────────────────────────────────────────────────────────
// Border checking
// ──────────────────────────────────────────────────────────────────────────────

// wouldOverflowEast checks if placing a tile at x going East would exceed the
// right boundary (MaxCols - Padding).
func (bl *BoardLayout) wouldOverflowEast(x int, width int) bool {
	return x+width > bl.MaxCols-bl.Padding
}

// wouldOverflowWest checks if placing a tile at x going West would exceed the
// left boundary (Padding).
func (bl *BoardLayout) wouldOverflowWest(x int, width int) bool {
	return x-width+1 < bl.Padding
}

// wouldOverflowSouth checks if placing a tile at y going South would exceed
// the bottom boundary.
func (bl *BoardLayout) wouldOverflowSouth(y int, height int) bool {
	return y+height > bl.MaxRows-bl.Padding
}

// wouldOverflowNorth checks if placing a tile at y going North would exceed
// the top boundary.
func (bl *BoardLayout) wouldOverflowNorth(y int, height int) bool {
	return y-height+1 < bl.Padding
}

// ──────────────────────────────────────────────────────────────────────────────
// Collision checking
// ──────────────────────────────────────────────────────────────────────────────

// cellsFree returns true if all cells in the rectangle [x, x+w) × [y, y+h)
// are within bounds and unoccupied.
func (bl *BoardLayout) cellsFree(x, y, w, h int) bool {
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			cx, cy := x+dx, y+dy
			if cx < 0 || cx >= bl.MaxCols || cy < 0 || cy >= bl.MaxRows {
				return false
			}
			if bl.Occupied[cy][cx] {
				return false
			}
		}
	}
	return true
}

// markOccupied marks cells in [x, x+w) × [y, y+h) as occupied.
func (bl *BoardLayout) markOccupied(x, y, w, h int) {
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			cx, cy := x+dx, y+dy
			if cx >= 0 && cx < bl.MaxCols && cy >= 0 && cy < bl.MaxRows {
				bl.Occupied[cy][cx] = true
			}
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Placement computation
// ──────────────────────────────────────────────────────────────────────────────

// nextStraight returns the (x, y) for placing the next tile in the same
// direction from anchor (prevX, prevY) with the previous tile's footprint.
func nextStraight(prevX, prevY, prevW, prevH int, dir Direction) (int, int) {
	switch dir {
	case East:
		return prevX + prevW, prevY
	case West:
		return prevX - 1, prevY // will adjust for width later
	case South:
		return prevX, prevY + prevH
	case North:
		return prevX, prevY - 1 // will adjust for height later
	}
	return prevX, prevY
}

// adjustForWidth adjusts x when flowing West (tile anchor is its left-most cell).
func adjustForWidth(x, width int, dir Direction) int {
	if dir == West {
		return x - width + 1
	}
	return x
}

// adjustForHeight adjusts y when flowing North.
func adjustForHeight(y, height int, dir Direction) int {
	if dir == North {
		return y - height + 1
	}
	return y
}

// ──────────────────────────────────────────────────────────────────────────────
// Core algorithm: CalculatePosition
// ──────────────────────────────────────────────────────────────────────────────

// PlacementResult describes where and how to render the new tile.
type PlacementResult struct {
	X         int       // grid X (column) of the tile's top-left cell
	Y         int       // grid Y (row) of the tile's top-left cell
	Width     int       // cells wide
	Height    int       // cells tall
	Direction Direction // the flow direction after placing this tile
	IsCurve   bool      // true if a forced curve was applied
}

// CalculatePosition receives the current board layout and a new tile, and
// returns the optimal (X, Y) position plus the updated flow direction so
// the chain stays within the canvas, serpentining as needed.
//
// Algorithm:
//  1. If the board is empty, place at the centre.
//  2. Try to continue straight in the current direction.
//  3. If straight would overflow → force a curve:
//     a. Move one step perpendicular (South preferred, else North).
//     b. Reverse the horizontal flow (East↔West).
//  4. Special bucha (double) handling near borders: the double itself is
//     rendered normally, but the returned Direction signals the next tile
//     to exit vertically (creating the "L" / "T" turn).
func (bl *BoardLayout) CalculatePosition(tile Tile, currentDir Direction) PlacementResult {
	isDouble := tile.IsDouble()

	// ── First tile: place in the centre of the grid. ─────────────────────
	if len(bl.Tiles) == 0 {
		startDir := currentDir
		w := tileWidth(isDouble, startDir)
		h := tileHeight(isDouble, startDir)
		x := (bl.MaxCols - w) / 2
		y := (bl.MaxRows - h) / 2
		bl.place(tile, x, y, w, h, startDir, false)
		return PlacementResult{X: x, Y: y, Width: w, Height: h, Direction: startDir}
	}

	prev := bl.Tiles[len(bl.Tiles)-1]

	prevW := tileWidth(prev.IsDouble, prev.Direction)
	prevH := tileHeight(prev.IsDouble, prev.Direction)

	// ── Attempt 1: continue straight ─────────────────────────────────────
	straightDir := currentDir
	w := tileWidth(isDouble, straightDir)
	h := tileHeight(isDouble, straightDir)

	sx, sy := nextStraight(prev.X, prev.Y, prevW, prevH, straightDir)
	sx = adjustForWidth(sx, w, straightDir)
	sy = adjustForHeight(sy, h, straightDir)

	if !bl.needsCurve(sx, sy, w, h, straightDir) && bl.cellsFree(sx, sy, w, h) {
		bl.place(tile, sx, sy, w, h, straightDir, false)
		return PlacementResult{X: sx, Y: sy, Width: w, Height: h, Direction: straightDir}
	}

	// ── Attempt 2: forced curve ──────────────────────────────────────────
	return bl.forceCurve(tile, prev, prevW, prevH, currentDir)
}

// needsCurve returns true if the candidate position is outside the safe zone.
func (bl *BoardLayout) needsCurve(x, y, w, h int, dir Direction) bool {
	switch dir {
	case East:
		return bl.wouldOverflowEast(x, w)
	case West:
		return bl.wouldOverflowWest(x+w-1, w) // x is left-most
	case South:
		return bl.wouldOverflowSouth(y, h)
	case North:
		return bl.wouldOverflowNorth(y+h-1, h)
	}
	return false
}

// forceCurve places the tile by stepping perpendicularly and reversing
// the horizontal flow.
//
// For horizontal flow (East/West):
//   - Step South (or North if South is blocked).
//   - Then flip East ↔ West.
//
// For vertical flow (South/North):
//   - Step East (or West if East is blocked).
//   - Then flip South ↔ North.
func (bl *BoardLayout) forceCurve(tile Tile, prev LayoutTile, prevW, prevH int, currentDir Direction) PlacementResult {
	isDouble := tile.IsDouble()

	// Determine the perpendicular step direction and the new primary direction.
	type candidate struct {
		perpDir Direction
		newDir  Direction
	}

	var candidates []candidate

	if currentDir.IsHorizontal() {
		// Try South first, then North.
		candidates = []candidate{
			{South, currentDir.Opposite()},
			{North, currentDir.Opposite()},
		}
	} else {
		// Vertical flow: try East first, then West.
		candidates = []candidate{
			{East, currentDir.Opposite()},
			{West, currentDir.Opposite()},
		}
	}

	for _, c := range candidates {
		// The "curve piece" goes in the perpendicular direction for one step,
		// then subsequent tiles continue in newDir.
		perpW := tileWidth(isDouble, c.perpDir)
		perpH := tileHeight(isDouble, c.perpDir)

		px, py := nextStraight(prev.X, prev.Y, prevW, prevH, c.perpDir)
		px = adjustForWidth(px, perpW, c.perpDir)
		py = adjustForHeight(py, perpH, c.perpDir)

		if bl.cellsFree(px, py, perpW, perpH) && !bl.outOfBounds(px, py, perpW, perpH) {
			bl.place(tile, px, py, perpW, perpH, c.newDir, true)
			return PlacementResult{
				X: px, Y: py,
				Width: perpW, Height: perpH,
				Direction: c.newDir,
				IsCurve:   true,
			}
		}
	}

	// ── Fallback: place adjacent to previous tile wherever possible. ─────
	return bl.fallbackPlace(tile, prev, prevW, prevH, currentDir)
}

// fallbackPlace tries all four directions as a last resort.
func (bl *BoardLayout) fallbackPlace(tile Tile, prev LayoutTile, prevW, prevH int, currentDir Direction) PlacementResult {
	isDouble := tile.IsDouble()
	dirs := []Direction{East, South, West, North}
	for _, d := range dirs {
		w := tileWidth(isDouble, d)
		h := tileHeight(isDouble, d)
		x, y := nextStraight(prev.X, prev.Y, prevW, prevH, d)
		x = adjustForWidth(x, w, d)
		y = adjustForHeight(y, h, d)
		if bl.cellsFree(x, y, w, h) && !bl.outOfBounds(x, y, w, h) {
			bl.place(tile, x, y, w, h, d, true)
			return PlacementResult{X: x, Y: y, Width: w, Height: h, Direction: d, IsCurve: true}
		}
	}
	// Absolute last resort: stack on the same cell (should never happen on a
	// reasonably sized board).
	w := tileWidth(isDouble, currentDir)
	h := tileHeight(isDouble, currentDir)
	bl.place(tile, prev.X, prev.Y, w, h, currentDir, true)
	return PlacementResult{X: prev.X, Y: prev.Y, Width: w, Height: h, Direction: currentDir, IsCurve: true}
}

// outOfBounds returns true if any part of the rectangle is outside the grid.
func (bl *BoardLayout) outOfBounds(x, y, w, h int) bool {
	return x < 0 || y < 0 || x+w > bl.MaxCols || y+h > bl.MaxRows
}

// place commits a tile's position to the layout.
func (bl *BoardLayout) place(tile Tile, x, y, w, h int, dir Direction, isCurve bool) {
	bl.markOccupied(x, y, w, h)
	bl.Tiles = append(bl.Tiles, LayoutTile{
		Tile:      tile,
		X:         x,
		Y:         y,
		Direction: dir,
		IsDouble:  tile.IsDouble(),
		IsCurve:   isCurve,
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Batch layout: recompute all positions from a chain of tiles.
// ──────────────────────────────────────────────────────────────────────────────

// LayoutChain takes a slice of tiles (in play order) and a grid size, and
// returns a fully computed BoardLayout with positions for every tile.
func LayoutChain(tiles []Tile, cols, rows, padding int) *BoardLayout {
	bl := NewBoardLayout(cols, rows, padding)
	dir := East
	for _, t := range tiles {
		res := bl.CalculatePosition(t, dir)
		dir = res.Direction
	}
	return bl
}
