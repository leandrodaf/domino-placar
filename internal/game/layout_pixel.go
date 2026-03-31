package game

// ──────────────────────────────────────────────────────────────────────────────
// Pixel-Space Snake Layout — UI rendering engine.
//
// Calculates absolute (X, Y) pixel positions and rotation angles for every
// domino tile so the chain serpentines ("ferradura / cobra") within the canvas.
// The frontend just reads X, Y, Rotation from each RenderedTile and applies
// them directly as CSS transforms: translate(X, Y) rotate(Rotation°).
//
// Coordinate contract
//   - (X, Y) = centre of the tile in pixels.
//   - Rotation = clockwise degrees: 0, 90, 180, 270.
//   - Horizontal tile (0°/180°): TileLength wide, TileWidth tall.
//   - Vertical tile  (90°/270°): TileWidth  wide, TileLength tall.
//
// Bidirectional vertical growth
//   The chain grows from a centre tile in two directions:
//     • Right side (East)  → curves go DOWN when hitting borders.
//     • Left side  (West)  → curves go UP   when hitting borders.
//   This prevents the two sides from colliding vertically.
//
// Snake / U-turn rules
//   The U-turn uses TWO vertical tiles to create a natural curve:
//     1. CurveEnter: first vertical tile, connected to last horizontal tile.
//        Direction is NOT inverted yet.
//     2. CurveExit: second vertical tile, placed directly above or below
//        CurveEnter depending on growDown. Direction IS inverted.
//   CurveExit rotation is INVERTED from CurveEnter so pips face the new row.
// ──────────────────────────────────────────────────────────────────────────────

const (
	DirEast = "E"
	DirWest = "W"
)

// CurveType identifies a tile's role in the U-turn sequence.
const (
	CurveNone  = 0
	CurveEnter = 1
	CurveExit  = 2
)

// BoardConfig describes the screen canvas and physical tile dimensions (pixels).
type BoardConfig struct {
	ScreenWidth  float64
	ScreenHeight float64
	TileLength   float64
	TileWidth    float64
	Padding      float64
}

// RenderedTile holds the computed pixel position and rotation for one tile.
type RenderedTile struct {
	Tile       Tile
	X          float64
	Y          float64
	Rotation   int // 0 | 90 | 180 | 270
	CurveType  int // CurveNone | CurveEnter | CurveExit
	RowCount   int // tiles placed in current row (resets after curve)
	RowDoubles int // doubles in current row (for max-tile adjustment)
}

// IsCurve returns true for any tile that is part of a U-turn.
func (rt RenderedTile) IsCurve() bool {
	return rt.CurveType == CurveEnter || rt.CurveType == CurveExit
}

// ──────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────────────────────────────────────

const borderEps = 0.5

func isVerticalRT(t RenderedTile) bool {
	return t.Rotation == 90 || t.Rotation == 270
}

// connectionTip returns the (x, y) pixel where the next tile will snap to the
// current tile when extending in direction dir.
//
// growDown controls the vertical direction of curves:
//
//	true  → curves descend (right side of chain)
//	false → curves ascend  (left side of chain)
func connectionTip(t RenderedTile, dir string, b BoardConfig, growDown bool) (x, y float64) {
	switch {
	case t.CurveType == CurveExit:
		rowDrop := b.TileLength/2 + b.TileWidth/2 + b.Padding
		var nextRowY float64
		if growDown {
			nextRowY = t.Y + rowDrop - b.TileWidth/2 + b.Padding/4
		} else {
			nextRowY = t.Y - rowDrop + b.TileWidth/2 - b.Padding/4
		}

		halfEdge := b.TileWidth / 2

		if dir == DirEast {
			return t.X - halfEdge, nextRowY
		}
		return t.X + halfEdge, nextRowY

	case isVerticalRT(t):
		if dir == DirEast {
			return t.X + b.TileWidth/2, t.Y
		}
		return t.X - b.TileWidth/2, t.Y

	default:
		if dir == DirEast {
			return t.X + b.TileLength/2, t.Y
		}
		return t.X - b.TileLength/2, t.Y
	}
}

// wouldOverflow reports whether a full-length horizontal tile placed from tipX
// would breach the screen border.
func wouldOverflow(tipX float64, dir string, b BoardConfig) bool {
	if dir == DirEast {
		return tipX+b.TileLength > b.ScreenWidth-b.Padding
	}
	return tipX-b.TileLength < b.Padding
}

func invertDir(dir string) string {
	if dir == DirEast {
		return DirWest
	}
	return DirEast
}

// ──────────────────────────────────────────────────────────────────────────────
// Placement sub-routines
// ──────────────────────────────────────────────────────────────────────────────

// placeRegular places a tile straight ahead — no U-turn needed.
func placeRegular(b BoardConfig, t Tile, tipX, tipY float64, dir string) (RenderedTile, string) {
	if t.IsDouble() {
		halfW := b.TileWidth / 2
		var x float64
		if dir == DirEast {
			x = tipX + halfW
		} else {
			x = tipX - halfW
		}
		return RenderedTile{Tile: t, X: x, Y: tipY, Rotation: 90}, dir
	}

	rotation := 0
	if dir == DirWest {
		rotation = 180
	}
	halfL := b.TileLength / 2
	var x float64
	if dir == DirEast {
		x = tipX + halfL
	} else {
		x = tipX - halfL
	}
	return RenderedTile{Tile: t, X: x, Y: tipY, Rotation: rotation}, dir
}

// placeCurveEnter places the FIRST tile of the U-turn.
// The tile is shifted vertically by TileWidth/2 so it starts flush with the
// edge of the horizontal row instead of being center-aligned.
//
// Non-doubles: vertical (90°/270°) — parallel to the curve column.
// Doubles:     horizontal (0°/180°) — perpendicular to the curve direction,
// keeping the domino convention that doubles are always perpendicular.
func placeCurveEnter(b BoardConfig, t Tile, tipX, tipY float64, dir string, growDown bool) (RenderedTile, string) {
	halfW := b.TileWidth / 2
	halfL := b.TileLength / 2

	y := tipY
	if growDown {
		y += halfW + halfL
	} else {
		y -= halfW + halfL
	}

	var rotation int
	var x float64
	if dir == DirEast {
		x = tipX - halfW
		if x+halfW > b.ScreenWidth-b.Padding {
			x = b.ScreenWidth - b.Padding - halfW
		}
		rotation = 90
	} else {
		x = tipX + halfW
		if x-halfW < b.Padding {
			x = b.Padding + halfW
		}
		rotation = 270
	}

	return RenderedTile{Tile: t, X: x, Y: y, Rotation: rotation, CurveType: CurveEnter}, dir
}

// placeCurveExit places the SECOND tile of the U-turn.
// Placed directly above (growDown=false) or below (growDown=true) CurveEnter.
// Gap is always TileLength to keep layout stable.
//
// Rotation is determined by the NEW direction (after inverting):
//
//	Non-doubles: 90° (newDir=East) or 270° (newDir=West).
//	Doubles:     0°  (newDir=East) or 180° (newDir=West) — horizontal.
func placeCurveExit(b BoardConfig, prevCurve RenderedTile, t Tile, dir string, growDown bool) (RenderedTile, string) {
	newDir := invertDir(dir)
	x := prevCurve.X

	var y float64
	if growDown {
		y = prevCurve.Y + b.TileLength
	} else {
		y = prevCurve.Y - b.TileLength
	}

	var rotation int
	if newDir == DirEast {
		rotation = 90
	} else {
		rotation = 270
	}

	return RenderedTile{Tile: t, X: x, Y: y, Rotation: rotation, CurveType: CurveExit}, newDir
}

// ──────────────────────────────────────────────────────────────────────────────
// Public API
// ──────────────────────────────────────────────────────────────────────────────

// FirstRenderedTile places the first domino at the centre of the canvas.
func FirstRenderedTile(b BoardConfig, tile Tile) RenderedTile {
	doubles := 0
	rotation := 0
	if tile.IsDouble() {
		doubles = 1
		rotation = 90
	}

	return RenderedTile{
		Tile:       tile,
		X:          b.ScreenWidth / 2,
		Y:          b.ScreenHeight / 2,
		Rotation:   rotation,
		RowCount:   1,
		RowDoubles: doubles,
	}
}

// CalculateNextPosition computes the absolute X, Y and Rotation for newTile
// when it is appended to one end of the chain.
//
// growDown controls the vertical direction of U-turn curves:
//
//	true  → curves descend (use for right/East side)
//	false → curves ascend  (use for left/West side)
func CalculateNextPosition(b BoardConfig, currentEnd RenderedTile, newTile Tile, currentDirection string, growDown bool) (RenderedTile, string) {
	if currentEnd.CurveType == CurveEnter {
		rt, dir := placeCurveExit(b, currentEnd, newTile, currentDirection, growDown)
		rt.RowCount = 0
		rt.RowDoubles = 0

		return rt, dir
	}

	tipX, tipY := connectionTip(currentEnd, currentDirection, b, growDown)

	maxTiles := 5
	if currentEnd.RowDoubles >= 2 {
		maxTiles = 6
	}

	rowFull := currentEnd.RowCount >= maxTiles && currentEnd.CurveType != CurveExit

	if rowFull || wouldOverflow(tipX, currentDirection, b) {
		return placeCurveEnter(b, newTile, tipX, tipY, currentDirection, growDown)
	}

	// Double immediately after CurveExit: place HORIZONTALLY instead of
	// perpendicular. A perpendicular double at the same X as the curve
	// column looks like a third curve tile; laying it flat makes it clearly
	// the first tile of the new horizontal row.
	if currentEnd.CurveType == CurveExit && newTile.IsDouble() {
		rotation := 0
		if currentDirection == DirWest {
			rotation = 180
		}
		halfL := b.TileLength / 2
		var x float64
		if currentDirection == DirEast {
			x = tipX + halfL
		} else {
			x = tipX - halfL
		}

		rt := RenderedTile{Tile: newTile, X: x, Y: tipY, Rotation: rotation}
		rt.RowCount = 1
		rt.RowDoubles = 1

		return rt, currentDirection
	}

	// Doubles are narrower (TileWidth) than regular tiles (TileLength).
	// If placing a double perpendicular here would leave no room for a full
	// tile after it, treat the double as a curve tile instead of squeezing
	// it in at the border.
	if newTile.IsDouble() {
		var doubleNextTip float64
		if currentDirection == DirEast {
			doubleNextTip = tipX + b.TileWidth
		} else {
			doubleNextTip = tipX - b.TileWidth
		}

		if wouldOverflow(doubleNextTip, currentDirection, b) {
			return placeCurveEnter(b, newTile, tipX, tipY, currentDirection, growDown)
		}
	}

	rt, dir := placeRegular(b, newTile, tipX, tipY, currentDirection)
	rt.RowCount = currentEnd.RowCount + 1
	rt.RowDoubles = currentEnd.RowDoubles
	if newTile.IsDouble() {
		rt.RowDoubles++
	}

	if currentEnd.CurveType == CurveExit {
		rt.RowCount = 1
		rt.RowDoubles = 0
		if newTile.IsDouble() {
			rt.RowDoubles = 1
		}
	}

	return rt, dir
}

// RenderChain computes pixel positions for an entire ordered slice of tiles,
// starting from the centre and growing East (curves go down).
func RenderChain(b BoardConfig, tiles []Tile) []RenderedTile {
	if len(tiles) == 0 {
		return nil
	}

	result := make([]RenderedTile, 0, len(tiles))

	first := FirstRenderedTile(b, tiles[0])
	result = append(result, first)

	dir := DirEast
	current := first

	for _, t := range tiles[1:] {
		next, newDir := CalculateNextPosition(b, current, t, dir, true)
		result = append(result, next)
		current = next
		dir = newDir
	}

	return result
}

// calculateNextPositionFromPlaced is like CalculateNextPosition but takes a
// PlacedTile so it can apply the correct visual flip for horizontal and curve tiles.
//
// The game's Flipped flag is set assuming the chain's original direction
// (East for right side, West for left side). After a U-turn the direction
// reverses, so the flip must be compensated: when the current horizontal
// direction differs from the side's starting direction, Flipped is inverted.
func calculateNextPositionFromPlaced(b BoardConfig, currentEnd RenderedTile, newTile PlacedTile, dir string, growDown bool) (RenderedTile, string) {
	rt, newDir := CalculateNextPosition(b, currentEnd, newTile.Tile, dir, growDown)

	if newTile.Tile.IsDouble() {
		return rt, newDir
	}

	if !isVerticalRT(rt) {
		goingEast := rt.Rotation == 0
		sideStartsEast := growDown
		flipped := newTile.Flipped
		if goingEast != sideStartsEast {
			flipped = !flipped
		}

		if flipped {
			rt.Rotation = 180
		} else {
			rt.Rotation = 0
		}
	} else if rt.CurveType != CurveNone {
		flipped := newTile.Flipped
		if (rt.CurveType == CurveEnter && dir == DirWest) || (rt.CurveType == CurveExit && dir == DirEast) {
			flipped = !flipped
		}

		if flipped {
			if rt.Rotation == 90 {
				rt.Rotation = 270
			} else {
				rt.Rotation = 90
			}
		}
	}

	return rt, newDir
}

// RenderBidirectionalChain computes pixel positions for a domino chain that
// grows in both directions from a centre tile.
//
// Right side (East) curves go DOWN. Left side (West) curves go UP.
// This prevents the two halves from colliding vertically.
func RenderBidirectionalChain(b BoardConfig, chain []PlacedTile, centerIdx int) []RenderedTile {
	if len(chain) == 0 {
		return nil
	}

	if centerIdx < 0 {
		centerIdx = 0
	}
	if centerIdx >= len(chain) {
		centerIdx = len(chain) - 1
	}

	result := make([]RenderedTile, len(chain))

	cp := chain[centerIdx]
	centerRotation := 0
	if cp.Tile.IsDouble() {
		centerRotation = 90
	} else if cp.Flipped {
		centerRotation = 180
	}
	doubles := 0
	if cp.Tile.IsDouble() {
		doubles = 1
	}
	centerRT := RenderedTile{
		Tile:       cp.Tile,
		X:          b.ScreenWidth / 2,
		Y:          b.ScreenHeight / 2,
		Rotation:   centerRotation,
		RowCount:   3,
		RowDoubles: doubles,
	}
	result[centerIdx] = centerRT

	// Right portion: grows East, curves go DOWN.
	dir := DirEast
	cur := centerRT
	for i := centerIdx + 1; i < len(chain); i++ {
		rt, newDir := calculateNextPositionFromPlaced(b, cur, chain[i], dir, true)
		result[i] = rt
		cur = rt
		dir = newDir
	}

	// Left portion: grows West, curves go UP.
	dir = DirWest
	cur = centerRT
	for i := centerIdx - 1; i >= 0; i-- {
		rt, newDir := calculateNextPositionFromPlaced(b, cur, chain[i], dir, false)
		result[i] = rt
		cur = rt
		dir = newDir
	}

	return result
}

// RotateLayout applies a global rotation (0, 90, 180, 270) to every tile's
// position and rotation. The engine always calculates in East/West; this
// transforms the output into North/South or inverted layouts.
//
// engineCfg is the BoardConfig used by the engine (may have swapped W/H).
// screenCfg is the real screen dimensions for re-centering after rotation.
func RotateLayout(tiles []RenderedTile, engineCfg, screenCfg BoardConfig, angle int) {
	if angle == 0 || len(tiles) == 0 {
		return
	}

	ecx := engineCfg.ScreenWidth / 2
	ecy := engineCfg.ScreenHeight / 2

	scx := screenCfg.ScreenWidth / 2
	scy := screenCfg.ScreenHeight / 2

	for i := range tiles {
		dx := tiles[i].X - ecx
		dy := tiles[i].Y - ecy

		switch angle {
		case 90:
			tiles[i].X = scx - dy
			tiles[i].Y = scy + dx
		case 180:
			tiles[i].X = scx - dx
			tiles[i].Y = scy - dy
		case 270:
			tiles[i].X = scx + dy
			tiles[i].Y = scy - dx
		}

		tiles[i].Rotation = (tiles[i].Rotation + angle) % 360
	}
}
