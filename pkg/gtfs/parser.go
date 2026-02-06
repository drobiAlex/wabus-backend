package gtfs

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strconv"
	"time"

	"wabus/internal/domain"
)

type ParseResult struct {
	Routes      map[string]*domain.Route
	Shapes      map[string]*domain.Shape
	Stops       map[string]*domain.Stop
	RouteShapes map[string][]string // route_id -> []shape_id

	StopSchedules map[string][]*domain.StopTime // stop_id -> []StopTime
	StopLines     map[string][]*domain.StopLine // stop_id -> []StopLine
	Trips         map[string]*TripInfo          // trip_id -> TripInfo (internal)
}

type TripInfo struct {
	RouteID  string
	Headsign string
}

type Parser struct {
	logger *slog.Logger
}

func NewParser(logger *slog.Logger) *Parser {
	return &Parser{
		logger: logger.With("component", "gtfs_parser"),
	}
}

func (p *Parser) Parse(reader *zip.Reader) (*ParseResult, error) {
	totalStart := time.Now()
	p.logger.Info("starting GTFS parsing")

	result := &ParseResult{
		Routes:        make(map[string]*domain.Route),
		Shapes:        make(map[string]*domain.Shape),
		Stops:         make(map[string]*domain.Stop),
		RouteShapes:   make(map[string][]string),
		StopSchedules: make(map[string][]*domain.StopTime),
		StopLines:     make(map[string][]*domain.StopLine),
		Trips:         make(map[string]*TripInfo),
	}

	fileMap := make(map[string]*zip.File)
	for _, file := range reader.File {
		fileMap[file.Name] = file
		p.logger.Debug("found file in archive",
			"name", file.Name,
			"compressed_size", file.CompressedSize64,
			"uncompressed_size", file.UncompressedSize64,
		)
	}

	if file, ok := fileMap["routes.txt"]; ok {
		start := time.Now()
		p.logger.Debug("parsing routes.txt")
		if err := p.parseRoutes(file, result); err != nil {
			return nil, fmt.Errorf("parse routes: %w", err)
		}
		p.logger.Info("parsed routes.txt",
			"count", len(result.Routes),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}

	if file, ok := fileMap["shapes.txt"]; ok {
		start := time.Now()
		p.logger.Debug("parsing shapes.txt")
		if err := p.parseShapes(file, result); err != nil {
			return nil, fmt.Errorf("parse shapes: %w", err)
		}
		totalPoints := 0
		for _, s := range result.Shapes {
			totalPoints += len(s.Points)
		}
		p.logger.Info("parsed shapes.txt",
			"shapes_count", len(result.Shapes),
			"total_points", totalPoints,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}

	if file, ok := fileMap["stops.txt"]; ok {
		start := time.Now()
		p.logger.Debug("parsing stops.txt")
		if err := p.parseStops(file, result); err != nil {
			return nil, fmt.Errorf("parse stops: %w", err)
		}
		p.logger.Info("parsed stops.txt",
			"count", len(result.Stops),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}

	if file, ok := fileMap["trips.txt"]; ok {
		start := time.Now()
		p.logger.Debug("parsing trips.txt")
		if err := p.parseTrips(file, result); err != nil {
			return nil, fmt.Errorf("parse trips: %w", err)
		}
		p.logger.Info("parsed trips.txt",
			"trips_count", len(result.Trips),
			"route_shapes_count", len(result.RouteShapes),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}

	if file, ok := fileMap["stop_times.txt"]; ok {
		start := time.Now()
		p.logger.Debug("parsing stop_times.txt (this may take a while)")
		if err := p.parseStopTimes(file, result); err != nil {
			return nil, fmt.Errorf("parse stop_times: %w", err)
		}
		totalStopTimes := 0
		for _, st := range result.StopSchedules {
			totalStopTimes += len(st)
		}
		p.logger.Info("parsed stop_times.txt",
			"stops_with_schedules", len(result.StopSchedules),
			"total_stop_times", totalStopTimes,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}

	start := time.Now()
	p.logger.Debug("building stop lines index")
	p.buildStopLines(result)
	p.logger.Info("built stop lines index",
		"stops_with_lines", len(result.StopLines),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	p.logger.Info("GTFS parsing completed",
		"total_duration_ms", time.Since(totalStart).Milliseconds(),
		"routes", len(result.Routes),
		"shapes", len(result.Shapes),
		"stops", len(result.Stops),
		"trips", len(result.Trips),
	)

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

		tripID := getField(record, idx, "trip_id")
		routeID := getField(record, idx, "route_id")
		shapeID := getField(record, idx, "shape_id")
		headsign := getField(record, idx, "trip_headsign")

		if tripID != "" && routeID != "" {
			result.Trips[tripID] = &TripInfo{
				RouteID:  routeID,
				Headsign: headsign,
			}
		}

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

func (p *Parser) parseStopTimes(file *zip.File, result *ParseResult) error {
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

		tripID := getField(record, idx, "trip_id")
		stopID := getField(record, idx, "stop_id")
		arrivalTime := getField(record, idx, "arrival_time")
		departureTime := getField(record, idx, "departure_time")
		stopSeq, _ := strconv.Atoi(getField(record, idx, "stop_sequence"))

		trip, ok := result.Trips[tripID]
		if !ok {
			continue
		}

		route, ok := result.Routes[trip.RouteID]
		if !ok {
			continue
		}

		stopTime := &domain.StopTime{
			TripID:        tripID,
			RouteID:       trip.RouteID,
			Line:          route.ShortName,
			Headsign:      trip.Headsign,
			ArrivalTime:   arrivalTime,
			DepartureTime: departureTime,
			StopSequence:  stopSeq,
		}

		result.StopSchedules[stopID] = append(result.StopSchedules[stopID], stopTime)
	}

	return nil
}

func (p *Parser) buildStopLines(result *ParseResult) {
	for stopID, stopTimes := range result.StopSchedules {
		lineMap := make(map[string]*domain.StopLine)
		headsignMap := make(map[string]map[string]bool)

		for _, st := range stopTimes {
			if _, exists := lineMap[st.RouteID]; !exists {
				route := result.Routes[st.RouteID]
				if route == nil {
					continue
				}
				lineMap[st.RouteID] = &domain.StopLine{
					RouteID:  st.RouteID,
					Line:     route.ShortName,
					LongName: route.LongName,
					Type:     route.Type,
					Color:    route.Color,
				}
				headsignMap[st.RouteID] = make(map[string]bool)
			}

			if st.Headsign != "" && !headsignMap[st.RouteID][st.Headsign] {
				headsignMap[st.RouteID][st.Headsign] = true
				lineMap[st.RouteID].Headsigns = append(lineMap[st.RouteID].Headsigns, st.Headsign)
			}
		}

		lines := make([]*domain.StopLine, 0, len(lineMap))
		for _, line := range lineMap {
			lines = append(lines, line)
		}

		sort.Slice(lines, func(i, j int) bool {
			return lines[i].Line < lines[j].Line
		})

		result.StopLines[stopID] = lines
	}
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
