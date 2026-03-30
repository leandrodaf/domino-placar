package game

// ──────────────────────────────────────────────────────────────────────────────
// Pixel-Space Snake Layout — UI rendering engine.
//
// Calculates absolute (X, Y) pixel positions and rotation angles for every
// domino tile so the chain serpentines ("ferradura / cobra") within the canvas
// without overflowing the screen. The frontend performs no calculations: it
// just reads X, Y, Rotation from each RenderedTile and applies them directly
// as CSS/SVG transforms (translate + rotate).
//
// Coordinate contract
//   • (X, Y)   = centre of the tile in pixels.
//   • Rotation = clockwise degrees: 0, 90, 180, 270.
//   • Horizontal tile (0°/180°): TileLength wide, TileWidth tall.
//   • Vertical tile  (90°/270°): TileWidth  wide, TileLength tall.
//
// Rotation semantics
//   0°   → horizontal, East flow (left→right, normal reading order).
//   90°  → vertical; perpendicular double OR right-border curve corner.
//   180° → horizontal, West flow (right→left; pips visually correct).
//   270° → vertical; left-border curve corner.
//
// Snake / U-turn rules
//   Before placing a tile in direction D, we check whether a full-length
//   horizontal tile would overflow (tipX ± TileLength crosses the border).
//   If it would:
//     a. If the new tile is a double (carroça): render it flat/horizontal as
//        the "lateral wall", saving TileLength-TileWidth of vertical space.
//     b. Otherwise: rotate it vertical (90° right / 270° left), snap its
//        outer edge to the border.
//   Either way, the direction is inverted (East ↔ West).
//   The row drop is implicit: connectionTip() returns the next-row Y for any
//   tile that is snapped to a border, so no explicit row counter is needed.
// ──────────────────────────────────────────────────────────────────────────────

const (
	DirEast = "E"
	DirWest = "W"
)

// BoardConfig describes the screen canvas and physical tile dimensions (pixels).
type BoardConfig struct {
	ScreenWidth  float64 // total canvas width
	ScreenHeight float64 // total canvas height
	TileLength   float64 // long  dimension of one tile face
	TileWidth    float64 // short dimension of one tile face
	Padding      float64 // minimum margin from each screen edge
}

// RenderedTile holds the computed pixel position and rotation for one tile.
// (X, Y) is the tile's centre. The game-logic layer uses PlacedTile (tile.go);
// RenderedTile is exclusively for the UI/render pipeline.
type RenderedTile struct {
	Tile     Tile
	X        float64
	Y        float64
	Rotation int // 0 | 90 | 180 | 270
}

// ──────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────────────────────────────────────

// borderEps is the floating-point tolerance for edge-alignment checks.
const borderEps = 0.5

func isVerticalRT(t RenderedTile) bool {
	return t.Rotation == 90 || t.Rotation == 270
}

// isVerticalCurveCorner returns true when a vertical tile is snapped flush
// against the right or left screen border — meaning it is a U-turn corner
// piece, not a regular perpendicular double in the middle of the row.
//
// This distinction drives the row-drop logic in connectionTip.
func isVerticalCurveCorner(t RenderedTile, b BoardConfig) bool {
	if !isVerticalRT(t) {
		return false
	}
	rightEdge := t.X + b.TileWidth/2
	leftEdge := t.X - b.TileWidth/2
	return rightEdge >= b.ScreenWidth-b.Padding-borderEps ||
		leftEdge <= b.Padding+borderEps
}

// isHorizontalWallDouble returns true when a non-rotated double tile is
// snapped against a border, acting as the "lateral wall" of the U-turn.
func isHorizontalWallDouble(t RenderedTile, b BoardConfig) bool {
	if !t.Tile.IsDouble() || isVerticalRT(t) {
		return false
	}
	rightEdge := t.X + b.TileLength/2
	leftEdge := t.X - b.TileLength/2
	return rightEdge >= b.ScreenWidth-b.Padding-borderEps ||
		leftEdge <= b.Padding+borderEps
}

// connectionTip returns the (x, y) pixel where the next tile will snap to the
// current tile when extending in direction dir.
//
// Four cases:
//
//	1. Vertical curve corner at border →
//	   next row is (TileLength + Padding) below; tip points there.
//	2. Horizontal wall-double at border (special carroça rule) →
//	   next row is only (TileWidth + Padding) below (space saving).
//	3. Vertical double mid-row →
//	   tip is the centre of the tile's leading edge at the same Y.
//	4. Regular horizontal tile →
//	   tip is the centre of its leading edge at the same Y.
func connectionTip(t RenderedTile, dir string, b BoardConfig) (x, y float64) {
	switch {
	case isVerticalCurveCorner(t, b):
		nextRowY := t.Y + b.TileLength + b.Padding
		if dir == DirEast {
			return t.X + b.TileWidth/2, nextRowY
		}
		return t.X - b.TileWidth/2, nextRowY

	case isHorizontalWallDouble(t, b):
		nextRowY := t.Y + b.TileWidth + b.Padding
		if dir == DirEast {
			return t.X + b.TileLength/2, nextRowY
		}
		return t.X - b.TileLength/2, nextRowY

	case isVerticalRT(t):
		// Perpendicular double in the middle of a row — horizontal flow continues.
		if dir == DirEast {
			return t.X + b.TileWidth/2, t.Y
		}
		return t.X - b.TileWidth/2, t.Y

	default:
		// Standard horizontal tile.
		if dir == DirEast {
			return t.X + b.TileLength/2, t.Y
		}
		return t.X - b.TileLength/2, t.Y
	}
}

// wouldOverflow reports whether a full-length horizontal tile placed from tipX
// would breach the screen border. Matches the spec formula:
//
//	East : tipX + TileLength > ScreenWidth  − Padding
//	West : tipX − TileLength < Padding
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
//
//   - Doubles → vertical (90°), centred on the connection point.
//   - Normal tiles → horizontal; 180° flip applied when flowing West so that
//     the pip values face the correct adjacent tile visually.
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

// placeCurveOrWall places a tile at the border, executing the U-turn.
//
// Doubles (carroças): laid flat as the lateral wall (saves vertical space).
// Normal tiles     : rotated vertical, snapped flush to the border.
//
// The row drop is NOT applied here — it surfaces automatically on the next call
// when connectionTip detects the tile as a curve corner or wall double.
func placeCurveOrWall(b BoardConfig, t Tile, tipX, tipY float64, dir string) (RenderedTile, string) {
	newDir := invertDir(dir)

	if t.IsDouble() {
		// ── Special rule: carroça na borda → horizontal wall ─────────────
		var x float64
		rotation := 0
		if dir == DirEast {
			x = b.ScreenWidth - b.Padding - b.TileLength/2
			rotation = 0
		} else {
			x = b.Padding + b.TileLength/2
			rotation = 180
		}
		return RenderedTile{Tile: t, X: x, Y: tipY, Rotation: rotation}, newDir
	}

	// ── Normal curve corner ───────────────────────────────────────────────
	// Right border: Rotation=90  (tile stands up, outer edge → right wall).
	// Left  border: Rotation=270 (tile stands up, outer edge → left  wall).
	var x float64
	rotation := 90
	if dir == DirEast {
		x = b.ScreenWidth - b.Padding - b.TileWidth/2
		rotation = 90
	} else {
		x = b.Padding + b.TileWidth/2
		rotation = 270
	}
	return RenderedTile{Tile: t, X: x, Y: tipY, Rotation: rotation}, newDir
}

// ──────────────────────────────────────────────────────────────────────────────
// Public API
// ──────────────────────────────────────────────────────────────────────────────

// FirstRenderedTile places the very first domino tile at the centre of the
// canvas, horizontal (Rotation = 0). Both ends of the chain grow from here.
func FirstRenderedTile(b BoardConfig, tile Tile) RenderedTile {
	return RenderedTile{
		Tile:     tile,
		X:        b.ScreenWidth / 2,
		Y:        b.ScreenHeight / 2,
		Rotation: 0,
	}
}

// CalculateNextPosition computes the absolute X, Y and Rotation for newTile
// when it is appended to one end of the chain (identified by currentEnd and
// currentDirection). The result is ready for direct use by the frontend
// renderer — no further calculations are needed client-side.
//
// Parameters:
//
//	b                – screen and tile configuration
//	currentEnd       – last RenderedTile on the end being extended
//	newTile          – tile to place
//	currentDirection – DirEast ("E") or DirWest ("W")
//
// Returns:
//
//	RenderedTile – fully computed position, ready for rendering
//	string       – new direction after placement (same or inverted on a curve)
func CalculateNextPosition(b BoardConfig, currentEnd RenderedTile, newTile Tile, currentDirection string) (RenderedTile, string) {
	tipX, tipY := connectionTip(currentEnd, currentDirection, b)

	if wouldOverflow(tipX, currentDirection, b) {
		return placeCurveOrWall(b, newTile, tipX, tipY, currentDirection)
	}
	return placeRegular(b, newTile, tipX, tipY, currentDirection)
}

// RenderChain computes pixel positions for an entire ordered slice of tiles,
// starting from the centre and growing East. It is a convenience wrapper
// for scenarios where the full chain is known upfront (e.g., replay / spectator).
//
// Returns a slice of RenderedTile in the same order as the input tiles.
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
		next, newDir := CalculateNextPosition(b, current, t, dir)
		result = append(result, next)
		current = next
		dir = newDir
	}

	return result
}
