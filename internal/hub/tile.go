package hub

import (
	"fmt"
	"math"
)

// TileID calculates tile ID for given coordinates at specified zoom level
// Uses Web Mercator (slippy map) tile scheme
func TileID(lat, lon float64, zoom int) string {
	n := math.Pow(2, float64(zoom))
	x := int(math.Floor((lon + 180.0) / 360.0 * n))
	latRad := lat * math.Pi / 180.0
	y := int(math.Floor((1.0 - math.Log(math.Tan(latRad)+1.0/math.Cos(latRad))/math.Pi) / 2.0 * n))

	maxTile := int(n) - 1
	if x < 0 {
		x = 0
	}
	if x > maxTile {
		x = maxTile
	}
	if y < 0 {
		y = 0
	}
	if y > maxTile {
		y = maxTile
	}

	return fmt.Sprintf("%d/%d/%d", zoom, x, y)
}

// TileBounds returns bounding box for a tile (minLat, minLon, maxLat, maxLon)
func TileBounds(zoom, x, y int) (minLat, minLon, maxLat, maxLon float64) {
	n := math.Pow(2, float64(zoom))
	minLon = float64(x)/n*360.0 - 180.0
	maxLon = float64(x+1)/n*360.0 - 180.0

	minLatRad := math.Atan(math.Sinh(math.Pi * (1 - 2*float64(y+1)/n)))
	maxLatRad := math.Atan(math.Sinh(math.Pi * (1 - 2*float64(y)/n)))
	minLat = minLatRad * 180.0 / math.Pi
	maxLat = maxLatRad * 180.0 / math.Pi
	return
}

// ParseTileID extracts zoom, x, y from a tile ID string
func ParseTileID(tileID string) (zoom, x, y int, ok bool) {
	n, err := fmt.Sscanf(tileID, "%d/%d/%d", &zoom, &x, &y)
	if err != nil || n != 3 {
		return 0, 0, 0, false
	}
	return zoom, x, y, true
}

// AdjacentTiles returns the given tile plus its 8 neighbors
func AdjacentTiles(zoom, x, y int) []string {
	maxTile := int(math.Pow(2, float64(zoom))) - 1
	tiles := make([]string, 0, 9)

	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			nx, ny := x+dx, y+dy
			if nx < 0 || nx > maxTile || ny < 0 || ny > maxTile {
				continue
			}
			tiles = append(tiles, fmt.Sprintf("%d/%d/%d", zoom, nx, ny))
		}
	}
	return tiles
}

// TilesInBBox returns all tile IDs that intersect the given bounding box
func TilesInBBox(minLat, minLon, maxLat, maxLon float64, zoom int) []string {
	topLeft := TileID(maxLat, minLon, zoom)
	bottomRight := TileID(minLat, maxLon, zoom)

	z1, x1, y1, ok1 := ParseTileID(topLeft)
	z2, x2, y2, ok2 := ParseTileID(bottomRight)

	if !ok1 || !ok2 || z1 != z2 {
		return nil
	}

	var tiles []string
	for x := x1; x <= x2; x++ {
		for y := y1; y <= y2; y++ {
			tiles = append(tiles, fmt.Sprintf("%d/%d/%d", zoom, x, y))
		}
	}
	return tiles
}
