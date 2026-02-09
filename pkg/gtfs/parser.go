package gtfs

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"wabus/internal/domain"
)

type ParseResult struct {
	Routes      map[string]*domain.Route
	Shapes      map[string]*domain.Shape
	Stops       map[string]*domain.Stop
	RouteShapes map[string][]string // route_id -> []shape_id

	StopSchedules  map[string][]domain.StopTimeCompact // stop_id -> compact stop times
	StopLines      map[string][]*domain.StopLine       // stop_id -> []StopLine
	RouteStops     map[string][]*domain.Stop           // route_id -> []Stop (ordered)
	RouteTripTimes map[string][]*domain.TripTimeEntry  // route_id -> []TripTimeEntry
	Trips          []domain.TripMeta                   // indexed trip metadata
	Calendars      map[string]*domain.Calendar         // service_id -> Calendar
	CalendarDates  map[string][]*domain.CalendarDate   // service_id -> []CalendarDate

	tripIndex map[string]uint32 // trip_id -> index in Trips (parse-only)
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
		Routes:         make(map[string]*domain.Route),
		Shapes:         make(map[string]*domain.Shape),
		Stops:          make(map[string]*domain.Stop),
		RouteShapes:    make(map[string][]string),
		StopSchedules:  make(map[string][]domain.StopTimeCompact),
		StopLines:      make(map[string][]*domain.StopLine),
		RouteStops:     make(map[string][]*domain.Stop),
		RouteTripTimes: make(map[string][]*domain.TripTimeEntry),
		Trips:          make([]domain.TripMeta, 0, 300000),
		Calendars:      make(map[string]*domain.Calendar),
		CalendarDates:  make(map[string][]*domain.CalendarDate),
		tripIndex:      make(map[string]uint32, 300000),
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

	if file, ok := fileMap["calendar.txt"]; ok {
		start := time.Now()
		p.logger.Debug("parsing calendar.txt")
		if err := p.parseCalendar(file, result); err != nil {
			return nil, fmt.Errorf("parse calendar: %w", err)
		}
		p.logger.Info("parsed calendar.txt",
			"services_count", len(result.Calendars),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}

	if file, ok := fileMap["calendar_dates.txt"]; ok {
		start := time.Now()
		p.logger.Debug("parsing calendar_dates.txt")
		if err := p.parseCalendarDates(file, result); err != nil {
			return nil, fmt.Errorf("parse calendar_dates: %w", err)
		}
		totalExceptions := 0
		for _, dates := range result.CalendarDates {
			totalExceptions += len(dates)
		}
		p.logger.Info("parsed calendar_dates.txt",
			"services_with_exceptions", len(result.CalendarDates),
			"total_exceptions", totalExceptions,
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

	start = time.Now()
	p.logger.Debug("building route stops index")
	p.buildRouteStops(result)
	p.logger.Info("built route stops index",
		"routes_with_stops", len(result.RouteStops),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	start = time.Now()
	p.logger.Debug("building trip time ranges")
	p.buildTripTimeRanges(result)
	totalEntries := 0
	for _, entries := range result.RouteTripTimes {
		totalEntries += len(entries)
	}
	p.logger.Info("built trip time ranges",
		"routes_with_times", len(result.RouteTripTimes),
		"total_entries", totalEntries,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	// tripIndex is only needed while parsing stop_times.txt.
	// Drop it now to reduce retained heap before returning the parsed dataset.
	result.tripIndex = nil

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

	seenRouteShapes := make(map[string]map[string]bool)

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
		serviceID := getField(record, idx, "service_id")
		shapeID := getField(record, idx, "shape_id")
		headsign := getField(record, idx, "trip_headsign")

		if tripID != "" && routeID != "" {
			if _, exists := result.tripIndex[tripID]; !exists {
				tripIdx := uint32(len(result.Trips))
				result.tripIndex[tripID] = tripIdx
				result.Trips = append(result.Trips, domain.TripMeta{
					ID:        tripID,
					RouteID:   routeID,
					ServiceID: serviceID,
					ShapeID:   shapeID,
					Headsign:  headsign,
				})
			}
		}

		if routeID == "" || shapeID == "" {
			continue
		}

		if seenRouteShapes[routeID] == nil {
			seenRouteShapes[routeID] = make(map[string]bool)
		}

		if !seenRouteShapes[routeID][shapeID] {
			seenRouteShapes[routeID][shapeID] = true
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
	r.ReuseRecord = true
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return err
	}

	idx := makeIndex(header)
	start := time.Now()
	var rows, accepted uint64

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		rows++

		tripID := getField(record, idx, "trip_id")
		tripIdx, ok := result.tripIndex[tripID]
		if !ok {
			continue
		}

		stopID := getField(record, idx, "stop_id")
		if stopID == "" {
			continue
		}

		arrivalSeconds := parseGTFSTimeToSeconds(getField(record, idx, "arrival_time"))
		departureSeconds := parseGTFSTimeToSeconds(getField(record, idx, "departure_time"))
		stopSeq, _ := strconv.Atoi(getField(record, idx, "stop_sequence"))
		if stopSeq < 0 {
			stopSeq = 0
		}
		if stopSeq > 65535 {
			stopSeq = 65535
		}

		result.StopSchedules[stopID] = append(result.StopSchedules[stopID], domain.StopTimeCompact{
			TripIndex:        tripIdx,
			ArrivalSeconds:   uint32(arrivalSeconds),
			DepartureSeconds: uint32(departureSeconds),
			StopSequence:     uint16(stopSeq),
		})
		accepted++

		if rows%1000000 == 0 {
			p.logger.Info("parseStopTimes progress",
				"rows_read", rows,
				"rows_accepted", accepted,
				"unique_stops", len(result.StopSchedules),
				"elapsed_ms", time.Since(start).Milliseconds(),
			)
		}
	}

	p.logger.Info("parseStopTimes finished",
		"rows_read", rows,
		"rows_accepted", accepted,
		"unique_stops", len(result.StopSchedules),
		"elapsed_ms", time.Since(start).Milliseconds(),
	)

	return nil
}

func (p *Parser) parseCalendar(file *zip.File, result *ParseResult) error {
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

		serviceID := getField(record, idx, "service_id")
		if serviceID == "" {
			continue
		}

		calendar := &domain.Calendar{
			ServiceID: serviceID,
			Monday:    getField(record, idx, "monday") == "1",
			Tuesday:   getField(record, idx, "tuesday") == "1",
			Wednesday: getField(record, idx, "wednesday") == "1",
			Thursday:  getField(record, idx, "thursday") == "1",
			Friday:    getField(record, idx, "friday") == "1",
			Saturday:  getField(record, idx, "saturday") == "1",
			Sunday:    getField(record, idx, "sunday") == "1",
			StartDate: getField(record, idx, "start_date"),
			EndDate:   getField(record, idx, "end_date"),
		}

		result.Calendars[serviceID] = calendar
	}

	return nil
}

func (p *Parser) parseCalendarDates(file *zip.File, result *ParseResult) error {
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

		serviceID := getField(record, idx, "service_id")
		if serviceID == "" {
			continue
		}

		exceptionType, _ := strconv.Atoi(getField(record, idx, "exception_type"))

		calendarDate := &domain.CalendarDate{
			ServiceID:     serviceID,
			Date:          getField(record, idx, "date"),
			ExceptionType: exceptionType,
		}

		result.CalendarDates[serviceID] = append(result.CalendarDates[serviceID], calendarDate)
	}

	return nil
}

func (p *Parser) buildStopLines(result *ParseResult) {
	for stopID, stopTimes := range result.StopSchedules {
		lineMap := make(map[string]*domain.StopLine)
		headsignMap := make(map[string]map[string]bool)

		for _, st := range stopTimes {
			tripIdx := int(st.TripIndex)
			if tripIdx < 0 || tripIdx >= len(result.Trips) {
				continue
			}
			trip := result.Trips[tripIdx]
			routeID := trip.RouteID

			if _, exists := lineMap[routeID]; !exists {
				route := result.Routes[routeID]
				if route == nil {
					continue
				}
				lineMap[routeID] = &domain.StopLine{
					RouteID:  routeID,
					Line:     route.ShortName,
					LongName: route.LongName,
					Type:     route.Type,
					Color:    route.Color,
				}
				headsignMap[routeID] = make(map[string]bool)
			}

			if trip.Headsign != "" && !headsignMap[routeID][trip.Headsign] {
				headsignMap[routeID][trip.Headsign] = true
				lineMap[routeID].Headsigns = append(lineMap[routeID].Headsigns, trip.Headsign)
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

func (p *Parser) buildRouteStops(result *ParseResult) {
	// Collect unique stop IDs per route, tracking the lowest stop_sequence per stop.
	type stopEntry struct {
		stopID string
		minSeq int
	}
	routeStopMap := make(map[string]map[string]*stopEntry) // route_id -> stop_id -> entry

	for stopID, stopTimes := range result.StopSchedules {
		for _, st := range stopTimes {
			tripIdx := int(st.TripIndex)
			if tripIdx < 0 || tripIdx >= len(result.Trips) {
				continue
			}
			routeID := result.Trips[tripIdx].RouteID
			if routeID == "" {
				continue
			}

			if _, ok := routeStopMap[routeID]; !ok {
				routeStopMap[routeID] = make(map[string]*stopEntry)
			}
			existing, exists := routeStopMap[routeID][stopID]
			seq := int(st.StopSequence)
			if !exists || seq < existing.minSeq {
				routeStopMap[routeID][stopID] = &stopEntry{stopID: stopID, minSeq: seq}
			}
		}
	}

	for routeID, stopMap := range routeStopMap {
		entries := make([]*stopEntry, 0, len(stopMap))
		for _, e := range stopMap {
			entries = append(entries, e)
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].minSeq < entries[j].minSeq
		})

		stops := make([]*domain.Stop, 0, len(entries))
		for _, e := range entries {
			if stop, ok := result.Stops[e.stopID]; ok {
				stopCopy := *stop
				stops = append(stops, &stopCopy)
			}
		}
		result.RouteStops[routeID] = stops
	}
}

func (p *Parser) buildTripTimeRanges(result *ParseResult) {
	// Build per-trip time ranges from compact stop schedules.
	tripCount := len(result.Trips)
	if tripCount == 0 {
		return
	}

	minTime := make([]int, tripCount)
	maxTime := make([]int, tripCount)
	seen := make([]bool, tripCount)

	for _, stopTimes := range result.StopSchedules {
		for _, st := range stopTimes {
			tripIdx := int(st.TripIndex)
			if tripIdx < 0 || tripIdx >= tripCount {
				continue
			}

			dep := int(st.DepartureSeconds / 60)
			arr := int(st.ArrivalSeconds / 60)

			if seen[tripIdx] {
				if dep < minTime[tripIdx] {
					minTime[tripIdx] = dep
				}
				if arr > maxTime[tripIdx] {
					maxTime[tripIdx] = arr
				}
			} else {
				seen[tripIdx] = true
				minTime[tripIdx] = dep
				maxTime[tripIdx] = arr
			}
		}
	}

	// Map trips to route -> [{shape, service, start, end}]
	for idx, trip := range result.Trips {
		if trip.ShapeID == "" || !seen[idx] {
			continue
		}

		entry := &domain.TripTimeEntry{
			ShapeID:      trip.ShapeID,
			ServiceID:    trip.ServiceID,
			StartMinutes: minTime[idx],
			EndMinutes:   maxTime[idx],
		}

		result.RouteTripTimes[trip.RouteID] = append(result.RouteTripTimes[trip.RouteID], entry)
	}
}

func parseGTFSTimeToSeconds(timeStr string) int {
	parts := strings.Split(timeStr, ":")
	if len(parts) < 2 {
		return 0
	}

	hours, _ := strconv.Atoi(parts[0])
	minutes, _ := strconv.Atoi(parts[1])
	seconds := 0
	if len(parts) >= 3 {
		seconds, _ = strconv.Atoi(parts[2])
	}

	if hours < 0 {
		hours = 0
	}
	if minutes < 0 {
		minutes = 0
	}
	if seconds < 0 {
		seconds = 0
	}

	return hours*3600 + minutes*60 + seconds
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
