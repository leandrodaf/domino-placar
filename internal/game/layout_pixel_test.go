package game

import (
	"math"
	"testing"
)

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

// testBoard is a typical 800×600 canvas with 60×30 tiles and 20px edge padding.
func testBoard() BoardConfig {
	return BoardConfig{
		ScreenWidth:  800,
		ScreenHeight: 600,
		TileLength:   60,
		TileWidth:    30,
		Padding:      20,
	}
}

func almostEqual(a, b float64) bool { return math.Abs(a-b) < 0.5 }

func normalTile() Tile  { return Tile{High: 5, Low: 3} }
func doubleTile() Tile  { return Tile{High: 4, Low: 4} }

// ──────────────────────────────────────────────────────────────────────────────
// FirstRenderedTile
// ──────────────────────────────────────────────────────────────────────────────

func TestFirstTileAtCenter(t *testing.T) {
	b := testBoard()
	rt := FirstRenderedTile(b, normalTile())

	if !almostEqual(rt.X, 400) {
		t.Errorf("X: want 400, got %.1f", rt.X)
	}
	if !almostEqual(rt.Y, 300) {
		t.Errorf("Y: want 300, got %.1f", rt.Y)
	}
	if rt.Rotation != 0 {
		t.Errorf("Rotation: want 0, got %d", rt.Rotation)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Straight East
// ──────────────────────────────────────────────────────────────────────────────

func TestStraightEastNormalTile(t *testing.T) {
	b := testBoard()
	first := FirstRenderedTile(b, normalTile())
	// tip = 400 + 30 = 430; next centre = 430 + 30 = 460
	next, dir := CalculateNextPosition(b, first, normalTile(), DirEast)

	if dir != DirEast {
		t.Errorf("direction: want E, got %s", dir)
	}
	if !almostEqual(next.X, 460) {
		t.Errorf("X: want 460, got %.1f", next.X)
	}
	if !almostEqual(next.Y, 300) {
		t.Errorf("Y: want 300, got %.1f", next.Y)
	}
	if next.Rotation != 0 {
		t.Errorf("Rotation: want 0, got %d", next.Rotation)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Straight West — 180° flip
// ──────────────────────────────────────────────────────────────────────────────

func TestStraightWestNormalTile(t *testing.T) {
	b := testBoard()
	first := FirstRenderedTile(b, normalTile())
	// tip = 400 - 30 = 370; next centre = 370 - 30 = 340
	next, dir := CalculateNextPosition(b, first, normalTile(), DirWest)

	if dir != DirWest {
		t.Errorf("direction: want W, got %s", dir)
	}
	if !almostEqual(next.X, 340) {
		t.Errorf("X: want 340, got %.1f", next.X)
	}
	if !almostEqual(next.Y, 300) {
		t.Errorf("Y: want 300, got %.1f", next.Y)
	}
	// West direction must add 180° flip so pips face the correct neighbour.
	if next.Rotation != 180 {
		t.Errorf("Rotation: want 180, got %d", next.Rotation)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Perpendicular double (mid-row)
// ──────────────────────────────────────────────────────────────────────────────

func TestDoubleGoesPerpendicularEast(t *testing.T) {
	b := testBoard()
	first := FirstRenderedTile(b, normalTile())
	// tip = 430; double centre = 430 + TileWidth/2 = 445
	next, dir := CalculateNextPosition(b, first, doubleTile(), DirEast)

	if dir != DirEast {
		t.Errorf("direction: want E, got %s", dir)
	}
	if next.Rotation != 90 {
		t.Errorf("double Rotation: want 90, got %d", next.Rotation)
	}
	if !almostEqual(next.X, 445) {
		t.Errorf("X: want 445, got %.1f", next.X)
	}
	if !almostEqual(next.Y, 300) {
		t.Errorf("Y (same row): want 300, got %.1f", next.Y)
	}
}

func TestDoubleGoesPerpendicularWest(t *testing.T) {
	b := testBoard()
	first := FirstRenderedTile(b, normalTile())
	// tip = 370; double centre = 370 - TileWidth/2 = 355
	next, dir := CalculateNextPosition(b, first, doubleTile(), DirWest)

	if dir != DirWest {
		t.Errorf("direction: want W, got %s", dir)
	}
	if next.Rotation != 90 {
		t.Errorf("double Rotation: want 90, got %d", next.Rotation)
	}
	if !almostEqual(next.X, 355) {
		t.Errorf("X: want 355, got %.1f", next.X)
	}
}

// After a perpendicular double, the next normal tile continues at the same Y.
func TestTileAfterMidRowDouble(t *testing.T) {
	b := testBoard()
	first := FirstRenderedTile(b, normalTile())

	dbl, dir := CalculateNextPosition(b, first, doubleTile(), DirEast)
	next, _ := CalculateNextPosition(b, dbl, normalTile(), dir)

	if !almostEqual(next.Y, first.Y) {
		t.Errorf("Y after mid-row double: want %.1f, got %.1f", first.Y, next.Y)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// U-turn curve corner (East → hits right border → West)
// ──────────────────────────────────────────────────────────────────────────────

func TestCurveAtRightBorder(t *testing.T) {
	b := testBoard()
	// Overflow triggers when tipX + TileLength > ScreenWidth − Padding
	// i.e. tipX > 800 − 20 − 60 = 720.
	// Manufacture a currentEnd whose East tip would be 730 (> 720).
	// centre X = 730 − TileLength/2 = 700 → tip = 700 + 30 = 730.
	currentEnd := RenderedTile{Tile: normalTile(), X: 700, Y: 300, Rotation: 0}

	curved, newDir := CalculateNextPosition(b, currentEnd, normalTile(), DirEast)

	if newDir != DirWest {
		t.Errorf("direction after East curve: want W, got %s", newDir)
	}
	// Curve tile must be rotated vertical.
	if curved.Rotation != 90 {
		t.Errorf("curve tile Rotation: want 90, got %d", curved.Rotation)
	}
	// Curve tile's right edge must not exceed ScreenWidth − Padding.
	rightEdge := curved.X + b.TileWidth/2
	if rightEdge > b.ScreenWidth-b.Padding+borderEps {
		t.Errorf("curve tile overflows right border: rightEdge=%.1f, limit=%.1f",
			rightEdge, b.ScreenWidth-b.Padding)
	}
	// Y must stay on the same row.
	if !almostEqual(curved.Y, 300) {
		t.Errorf("curve tile Y: want 300 (same row), got %.1f", curved.Y)
	}
}

func TestCurveAtLeftBorder(t *testing.T) {
	b := testBoard()
	// Overflow triggers when tipX − TileLength < Padding.
	// i.e. tipX < 20 + 60 = 80. Use centre X=100 → tip = 100 − 30 = 70 < 80.
	currentEnd := RenderedTile{Tile: normalTile(), X: 100, Y: 300, Rotation: 180}

	curved, newDir := CalculateNextPosition(b, currentEnd, normalTile(), DirWest)

	if newDir != DirEast {
		t.Errorf("direction after West curve: want E, got %s", newDir)
	}
	if curved.Rotation != 270 {
		t.Errorf("curve tile Rotation: want 270, got %d", curved.Rotation)
	}
	leftEdge := curved.X - b.TileWidth/2
	if leftEdge < b.Padding-borderEps {
		t.Errorf("curve tile overflows left border: leftEdge=%.1f, limit=%.1f",
			leftEdge, b.Padding)
	}
	if !almostEqual(curved.Y, 300) {
		t.Errorf("curve tile Y: want 300, got %.1f", curved.Y)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Row drop: tile after a curve corner uses new Y
// ──────────────────────────────────────────────────────────────────────────────

func TestRowDropAfterRightCurve(t *testing.T) {
	b := testBoard()
	currentEnd := RenderedTile{Tile: normalTile(), X: 700, Y: 300, Rotation: 0}

	curveCorner, newDir := CalculateNextPosition(b, currentEnd, normalTile(), DirEast)
	// curveCorner.Y = 300. Next row Y = 300 + TileLength + Padding = 300+60+20=380.
	afterCurve, _ := CalculateNextPosition(b, curveCorner, normalTile(), newDir)

	expectedY := 300 + b.TileLength + b.Padding // 380
	if !almostEqual(afterCurve.Y, expectedY) {
		t.Errorf("Y after curve row-drop: want %.1f, got %.1f", expectedY, afterCurve.Y)
	}
}

func TestRowDropAfterLeftCurve(t *testing.T) {
	b := testBoard()
	currentEnd := RenderedTile{Tile: normalTile(), X: 100, Y: 380, Rotation: 180}

	curveCorner, newDir := CalculateNextPosition(b, currentEnd, normalTile(), DirWest)
	afterCurve, _ := CalculateNextPosition(b, curveCorner, normalTile(), newDir)

	expectedY := 380 + b.TileLength + b.Padding // 460
	if !almostEqual(afterCurve.Y, expectedY) {
		t.Errorf("Y after left-curve row-drop: want %.1f, got %.1f", expectedY, afterCurve.Y)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// 180° flip after direction inversion
// ──────────────────────────────────────────────────────────────────────────────

func TestNormalTileAfterCurveHas180(t *testing.T) {
	b := testBoard()
	currentEnd := RenderedTile{Tile: normalTile(), X: 700, Y: 300, Rotation: 0}

	curveCorner, newDir := CalculateNextPosition(b, currentEnd, normalTile(), DirEast)
	afterCurve, _ := CalculateNextPosition(b, curveCorner, normalTile(), newDir)

	if afterCurve.Rotation != 180 {
		t.Errorf("tile going West should have Rotation=180, got %d", afterCurve.Rotation)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Special rule: double (carroça) at the border → horizontal wall
// ──────────────────────────────────────────────────────────────────────────────

func TestDoubleAtRightBorderBecomesHorizontalWall(t *testing.T) {
	b := testBoard()
	currentEnd := RenderedTile{Tile: normalTile(), X: 700, Y: 300, Rotation: 0}

	wall, newDir := CalculateNextPosition(b, currentEnd, doubleTile(), DirEast)

	if newDir != DirWest {
		t.Errorf("direction after wall-double: want W, got %s", newDir)
	}
	// Must be horizontal (NOT perpendicular).
	if wall.Rotation == 90 || wall.Rotation == 270 {
		t.Errorf("border double must be horizontal, got Rotation=%d", wall.Rotation)
	}
	// Right edge must not exceed ScreenWidth − Padding.
	rightEdge := wall.X + b.TileLength/2
	if rightEdge > b.ScreenWidth-b.Padding+borderEps {
		t.Errorf("border double overflows: rightEdge=%.1f, limit=%.1f",
			rightEdge, b.ScreenWidth-b.Padding)
	}
}

func TestDoubleAtLeftBorderBecomesHorizontalWall(t *testing.T) {
	b := testBoard()
	currentEnd := RenderedTile{Tile: normalTile(), X: 100, Y: 300, Rotation: 180}

	wall, newDir := CalculateNextPosition(b, currentEnd, doubleTile(), DirWest)

	if newDir != DirEast {
		t.Errorf("direction after wall-double: want E, got %s", newDir)
	}
	if wall.Rotation == 90 || wall.Rotation == 270 {
		t.Errorf("border double must be horizontal, got Rotation=%d", wall.Rotation)
	}
	// Left edge must not go below Padding.
	leftEdge := wall.X - b.TileLength/2
	if leftEdge < b.Padding-borderEps {
		t.Errorf("border double overflows left: leftEdge=%.1f, limit=%.1f",
			leftEdge, b.Padding)
	}
	// West wall double has 180° rotation.
	if wall.Rotation != 180 {
		t.Errorf("West wall-double Rotation: want 180, got %d", wall.Rotation)
	}
}

// Wall-double row drop is smaller: TileWidth + Padding (not TileLength + Padding).
func TestRowDropAfterWallDouble(t *testing.T) {
	b := testBoard()
	currentEnd := RenderedTile{Tile: normalTile(), X: 700, Y: 300, Rotation: 0}

	wall, newDir := CalculateNextPosition(b, currentEnd, doubleTile(), DirEast)
	afterWall, _ := CalculateNextPosition(b, wall, normalTile(), newDir)

	// TileWidth=30, Padding=20 → next row at 300 + 30 + 20 = 350
	expectedY := 300 + b.TileWidth + b.Padding
	if !almostEqual(afterWall.Y, expectedY) {
		t.Errorf("Y after wall-double row-drop: want %.1f, got %.1f", expectedY, afterWall.Y)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// RenderChain — full serpentine with 15 tiles, no out-of-bounds
// ──────────────────────────────────────────────────────────────────────────────

func TestRenderChainNoBoundsViolation(t *testing.T) {
	b := testBoard()
	tiles := []Tile{
		{6, 5}, {5, 4}, {4, 3}, {3, 2}, {2, 1},
		{1, 0}, {0, 6}, {6, 3}, {3, 1}, {1, 5},
		{5, 2}, {2, 6}, {6, 6}, {6, 1}, {1, 4},
	}
	result := RenderChain(b, tiles)

	if len(result) != len(tiles) {
		t.Fatalf("want %d RenderedTiles, got %d", len(tiles), len(result))
	}

	right := b.ScreenWidth - b.Padding
	bottom := b.ScreenHeight - b.Padding

	for i, rt := range result {
		var hw, hh float64
		if isVerticalRT(rt) {
			hw, hh = b.TileWidth/2, b.TileLength/2
		} else {
			hw, hh = b.TileLength/2, b.TileWidth/2
		}

		if rt.X-hw < b.Padding-borderEps {
			t.Errorf("tile %d (%v) left edge %.1f < padding %.1f", i, rt.Tile, rt.X-hw, b.Padding)
		}
		if rt.X+hw > right+borderEps {
			t.Errorf("tile %d (%v) right edge %.1f > %.1f", i, rt.Tile, rt.X+hw, right)
		}
		if rt.Y-hh < b.Padding-borderEps {
			t.Errorf("tile %d (%v) top edge %.1f < padding %.1f", i, rt.Tile, rt.Y-hh, b.Padding)
		}
		if rt.Y+hh > bottom+borderEps {
			t.Errorf("tile %d (%v) bottom edge %.1f > %.1f", i, rt.Tile, rt.Y+hh, bottom)
		}
	}
}

func TestRenderChainFirstTileAtCenter(t *testing.T) {
	b := testBoard()
	tiles := []Tile{{6, 5}, {5, 4}}
	result := RenderChain(b, tiles)

	if !almostEqual(result[0].X, b.ScreenWidth/2) || !almostEqual(result[0].Y, b.ScreenHeight/2) {
		t.Errorf("first tile should be at centre (%.0f, %.0f), got (%.1f, %.1f)",
			b.ScreenWidth/2, b.ScreenHeight/2, result[0].X, result[0].Y)
	}
}

func TestRenderChainEmpty(t *testing.T) {
	result := RenderChain(testBoard(), nil)
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Rotation helpers
// ──────────────────────────────────────────────────────────────────────────────

func TestInvertDir(t *testing.T) {
	if invertDir(DirEast) != DirWest {
		t.Error("invertDir(E) should be W")
	}
	if invertDir(DirWest) != DirEast {
		t.Error("invertDir(W) should be E")
	}
}

func TestIsVerticalRT(t *testing.T) {
	for _, rot := range []int{90, 270} {
		if !isVerticalRT(RenderedTile{Rotation: rot}) {
			t.Errorf("Rotation=%d should be vertical", rot)
		}
	}
	for _, rot := range []int{0, 180} {
		if isVerticalRT(RenderedTile{Rotation: rot}) {
			t.Errorf("Rotation=%d should NOT be vertical", rot)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Full 28-tile serpentine — smoke test that nothing panics and all tiles fit
// ──────────────────────────────────────────────────────────────────────────────

func TestFullDominoSet28Tiles(t *testing.T) {
	b := BoardConfig{
		ScreenWidth:  1024,
		ScreenHeight: 768,
		TileLength:   60,
		TileWidth:    30,
		Padding:      20,
	}

	// All 28 tiles in a standard double-six set.
	var tiles []Tile
	for high := 0; high <= 6; high++ {
		for low := 0; low <= high; low++ {
			tiles = append(tiles, Tile{High: high, Low: low})
		}
	}

	result := RenderChain(b, tiles)
	if len(result) != 28 {
		t.Fatalf("want 28 RenderedTiles, got %d", len(result))
	}

	right := b.ScreenWidth - b.Padding
	bottom := b.ScreenHeight - b.Padding

	for i, rt := range result {
		var hw, hh float64
		if isVerticalRT(rt) {
			hw, hh = b.TileWidth/2, b.TileLength/2
		} else {
			hw, hh = b.TileLength/2, b.TileWidth/2
		}

		if rt.X-hw < b.Padding-borderEps || rt.X+hw > right+borderEps ||
			rt.Y-hh < b.Padding-borderEps || rt.Y+hh > bottom+borderEps {
			t.Errorf("tile %d (%v) out of bounds: centre=(%.1f, %.1f) rot=%d",
				i, rt.Tile, rt.X, rt.Y, rt.Rotation)
		}
	}
}
