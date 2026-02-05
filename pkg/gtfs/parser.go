package gtfs

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"

	"wabus/internal/domain"
)

type ParseResult struct {
	Routes      map[string]*domain.Route
	Shapes      map[string]*domain.Shape
	Stops       map[string]*domain.Stop
	RouteShapes map[string][]string // route_id -> []shape_id
}

type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) Parse(reader *zip.Reader) (*ParseResult, error) {
	result := &ParseResult{
		Routes:      make(map[string]*domain.Route),
		Shapes:      make(map[string]*domain.Shape),
		Stops:       make(map[string]*domain.Stop),
		RouteShapes: make(map[string][]string),
	}

	for _, file := range reader.File {
		switch file.Name {
		case "routes.txt":
			if err := p.parseRoutes(file, result); err != nil {
				return nil, fmt.Errorf("parse routes: %w", err)
			}
		case "shapes.txt":
			if err := p.parseShapes(file, result); err != nil {
				return nil, fmt.Errorf("parse shapes: %w", err)
			}
		case "stops.txt":
			if err := p.parseStops(file, result); err != nil {
				return nil, fmt.Errorf("parse stops: %w", err)
			}
		case "trips.txt":
			if err := p.parseTrips(file, result); err != nil {
				return nil, fmt.Errorf("parse trips: %w", err)
			}
		}
	}

	return result, nil
}

func (p *Parser) parseRoutes(file *zip.File, result *ParseResult) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	r := csv.NewReader(rc)
	header, err := r.Read()
	if err != nil {
		return err
	}

	idx := makeIndex(header)

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		routeType := 3
		if v := getField(record, idx, "route_type"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil {
				routeType = parsed
			}
		}

		route := &domain.Route{
			ID:        getField(record, idx, "route_id"),
			ShortName: getField(record, idx, "route_short_name"),
			LongName:  getField(record, idx, "route_long_name"),
			Type:      domain.RouteType(routeType),
			Color:     getField(record, idx, "route_color"),
			TextColor: getField(record, idx, "route_text_color"),
		}

		result.Routes[route.ID] = route
	}

	return nil
}

func (p *Parser) parseShapes(file *zip.File, result *ParseResult) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	r := csv.NewReader(rc)
	header, err := r.Read()
	if err != nil {
		return err
	}

	idx := makeIndex(header)

	points := make(map[string][]domain.ShapePoint)

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		shapeID := getField(record, idx, "shape_id")

		lat, _ := strconv.ParseFloat(getField(record, idx, "shape_pt_lat"), 64)
		lon, _ := strconv.ParseFloat(getField(record, idx, "shape_pt_lon"), 64)
		seq, _ := strconv.Atoi(getField(record, idx, "shape_pt_sequence"))

		points[shapeID] = append(points[shapeID], domain.ShapePoint{
			Lat:      lat,
			Lon:      lon,
			Sequence: seq,
		})
	}

	for shapeID, pts := range points {
		sort.Slice(pts, func(i, j int) bool {
			return pts[i].Sequence < pts[j].Sequence
		})

		result.Shapes[shapeID] = &domain.Shape{
			ID:     shapeID,
			Points: pts,
		}
	}

	return nil
}

func (p *Parser) parseStops(file *zip.File, result *ParseResult) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	r := csv.NewReader(rc)
	header, err := r.Read()
	if err != nil {
		return err
	}

	idx := makeIndex(header)

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		lat, _ := strconv.ParseFloat(getField(record, idx, "stop_lat"), 64)
		lon, _ := strconv.ParseFloat(getField(record, idx, "stop_lon"), 64)

		stop := &domain.Stop{
			ID:   getField(record, idx, "stop_id"),
			Code: getField(record, idx, "stop_code"),
			Name: getField(record, idx, "stop_name"),
			Lat:  lat,
			Lon:  lon,
			Zone: getField(record, idx, "zone_id"),
		}

		result.Stops[stop.ID] = stop
	}

	return nil
}

func (p *Parser) parseTrips(file *zip.File, result *ParseResult) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	r := csv.NewReader(rc)
	header, err := r.Read()
	if err != nil {
		return err
	}

	idx := makeIndex(header)

	seen := make(map[string]map[string]bool)

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		routeID := getField(record, idx, "route_id")
		shapeID := getField(record, idx, "shape_id")

		if routeID == "" || shapeID == "" {
			continue
		}

		if seen[routeID] == nil {
			seen[routeID] = make(map[string]bool)
		}

		if !seen[routeID][shapeID] {
			seen[routeID][shapeID] = true
			result.RouteShapes[routeID] = append(result.RouteShapes[routeID], shapeID)
		}
	}

	return nil
}

func makeIndex(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, name := range header {
		idx[name] = i
	}
	return idx
}

func getField(record []string, idx map[string]int, field string) string {
	if i, ok := idx[field]; ok && i < len(record) {
		return record[i]
	}
	return ""
}
