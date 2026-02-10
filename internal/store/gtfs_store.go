package store

import (
	"fmt"
	"sync"
	"time"

	"wabus/internal/domain"
)

type GTFSStore struct {
	mu              sync.RWMutex
	routes          map[string]*domain.Route
	routesByLine    map[string]*domain.Route
	shapes          map[string]*domain.Shape
	routeShapes     map[string][]string
	stops           map[string]*domain.Stop
	routeStops      map[string][]*domain.Stop
	routeTripTimes  map[string][]*domain.TripTimeEntry
	stopSchedules   map[string][]domain.StopTimeCompact
	stopLines       map[string][]*domain.StopLine
	trips           []domain.TripMeta
	calendars       map[string]*domain.Calendar
	calendarDates   map[string][]*domain.CalendarDate
	shapeDirections map[string]int

	lastUpdate time.Time
}

func NewGTFSStore() *GTFSStore {
	return &GTFSStore{
		routes:          make(map[string]*domain.Route),
		routesByLine:    make(map[string]*domain.Route),
		shapes:          make(map[string]*domain.Shape),
		routeShapes:     make(map[string][]string),
		stops:           make(map[string]*domain.Stop),
		routeStops:      make(map[string][]*domain.Stop),
		routeTripTimes:  make(map[string][]*domain.TripTimeEntry),
		stopSchedules:   make(map[string][]domain.StopTimeCompact),
		stopLines:       make(map[string][]*domain.StopLine),
		trips:           make([]domain.TripMeta, 0),
		calendars:       make(map[string]*domain.Calendar),
		calendarDates:   make(map[string][]*domain.CalendarDate),
		shapeDirections: make(map[string]int),
	}
}

func (s *GTFSStore) UpdateAll(routes map[string]*domain.Route, shapes map[string]*domain.Shape, stops map[string]*domain.Stop, routeShapes map[string][]string, stopSchedules map[string][]domain.StopTimeCompact, stopLines map[string][]*domain.StopLine, routeStops map[string][]*domain.Stop, routeTripTimes map[string][]*domain.TripTimeEntry, trips []domain.TripMeta, calendars map[string]*domain.Calendar, calendarDates map[string][]*domain.CalendarDate, shapeDirections map[string]int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.routes = routes
	s.shapes = shapes
	s.stops = stops
	s.routeShapes = routeShapes
	s.stopSchedules = stopSchedules
	s.stopLines = stopLines
	s.routeStops = routeStops
	s.routeTripTimes = routeTripTimes
	s.trips = trips
	s.calendars = calendars
	s.calendarDates = calendarDates
	s.shapeDirections = shapeDirections
	s.lastUpdate = time.Now()

	s.routesByLine = make(map[string]*domain.Route, len(routes))
	for _, route := range routes {
		s.routesByLine[route.ShortName] = route
	}
}

func (s *GTFSStore) GetAllRoutes() []*domain.Route {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*domain.Route, 0, len(s.routes))
	for _, route := range s.routes {
		copy := *route
		result = append(result, &copy)
	}
	return result
}

func (s *GTFSStore) GetRouteByID(id string) (*domain.Route, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	route, ok := s.routes[id]
	if !ok {
		return nil, false
	}
	copy := *route
	return &copy, true
}

func (s *GTFSStore) GetRouteByLine(line string) (*domain.Route, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	route, ok := s.routesByLine[line]
	if !ok {
		return nil, false
	}
	copy := *route
	return &copy, true
}

func (s *GTFSStore) GetRouteShapes(routeID string) []*domain.Shape {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getRouteShapesLocked(routeID)
}

func (s *GTFSStore) GetActiveRouteShapes(routeID string, date time.Time, timeMinutes int) []*domain.Shape {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tripTimes, ok := s.routeTripTimes[routeID]
	if !ok {
		return s.getRouteShapesLocked(routeID)
	}

	dateStr := date.Format("20060102")
	weekday := date.Weekday()
	activeServices := s.getActiveServices(dateStr, weekday)

	// Also check yesterday for after-midnight GTFS trips (times > 24:00)
	yesterday := date.AddDate(0, 0, -1)
	yesterdayStr := yesterday.Format("20060102")
	yesterdayWeekday := yesterday.Weekday()
	yesterdayServices := s.getActiveServices(yesterdayStr, yesterdayWeekday)

	activeShapeIDs := make(map[string]bool)

	for _, tt := range tripTimes {
		// Check today's services
		if activeServices[tt.ServiceID] {
			if tt.StartMinutes <= timeMinutes+30 && tt.EndMinutes >= timeMinutes-30 {
				activeShapeIDs[tt.ShapeID] = true
			}
		}
		// Check yesterday's services for after-midnight trips
		if yesterdayServices[tt.ServiceID] && tt.EndMinutes > 1440 {
			adjustedTime := timeMinutes + 1440
			if tt.StartMinutes <= adjustedTime+30 && tt.EndMinutes >= adjustedTime-30 {
				activeShapeIDs[tt.ShapeID] = true
			}
		}
	}

	if len(activeShapeIDs) == 0 {
		return s.getRouteShapesLocked(routeID)
	}

	var result []*domain.Shape
	for shapeID := range activeShapeIDs {
		if shape, ok := s.shapes[shapeID]; ok {
			dir := s.shapeDirections[shapeID]
			shapeCopy := &domain.Shape{
				ID:          shape.ID,
				Points:      make([]domain.ShapePoint, len(shape.Points)),
				DirectionID: &dir,
			}
			copy(shapeCopy.Points, shape.Points)
			result = append(result, shapeCopy)
		}
	}
	return result
}

func (s *GTFSStore) getRouteShapesLocked(routeID string) []*domain.Shape {
	shapeIDs, ok := s.routeShapes[routeID]
	if !ok {
		return nil
	}
	result := make([]*domain.Shape, 0, len(shapeIDs))
	for _, shapeID := range shapeIDs {
		if shape, ok := s.shapes[shapeID]; ok {
			dir := s.shapeDirections[shapeID]
			shapeCopy := &domain.Shape{
				ID:          shape.ID,
				Points:      make([]domain.ShapePoint, len(shape.Points)),
				DirectionID: &dir,
			}
			copy(shapeCopy.Points, shape.Points)
			result = append(result, shapeCopy)
		}
	}
	return result
}

func (s *GTFSStore) GetAllStops() []*domain.Stop {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*domain.Stop, 0, len(s.stops))
	for _, stop := range s.stops {
		copy := *stop
		result = append(result, &copy)
	}
	return result
}

func (s *GTFSStore) GetStopByID(id string) (*domain.Stop, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stop, ok := s.stops[id]
	if !ok {
		return nil, false
	}
	copy := *stop
	return &copy, true
}

func (s *GTFSStore) GetRouteStops(routeID string) []*domain.Stop {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stops, ok := s.routeStops[routeID]
	if !ok {
		return nil
	}

	result := make([]*domain.Stop, len(stops))
	for i, stop := range stops {
		copy := *stop
		result[i] = &copy
	}
	return result
}

func (s *GTFSStore) GetStopSchedule(stopID string) []*domain.StopTime {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schedule, ok := s.stopSchedules[stopID]
	if !ok {
		return nil
	}

	result := make([]*domain.StopTime, 0, len(schedule))
	for _, st := range schedule {
		decoded, ok := s.decodeStopTimeLocked(st)
		if ok {
			result = append(result, decoded)
		}
	}
	return result
}

func (s *GTFSStore) GetStopScheduleForDate(stopID string, date time.Time) []*domain.StopTime {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schedule, ok := s.stopSchedules[stopID]
	if !ok {
		return nil
	}

	dateStr := date.Format("20060102")
	weekday := date.Weekday()
	activeServices := s.getActiveServices(dateStr, weekday)

	result := make([]*domain.StopTime, 0, len(schedule))
	for _, st := range schedule {
		tripIdx := int(st.TripIndex)
		if tripIdx < 0 || tripIdx >= len(s.trips) {
			continue
		}
		trip := s.trips[tripIdx]
		if !activeServices[trip.ServiceID] {
			continue
		}

		decoded, ok := s.decodeStopTimeLocked(st)
		if ok {
			result = append(result, decoded)
		}
	}
	return result
}

func (s *GTFSStore) decodeStopTimeLocked(st domain.StopTimeCompact) (*domain.StopTime, bool) {
	tripIdx := int(st.TripIndex)
	if tripIdx < 0 || tripIdx >= len(s.trips) {
		return nil, false
	}
	trip := s.trips[tripIdx]

	line := ""
	if route, ok := s.routes[trip.RouteID]; ok {
		line = route.ShortName
	}

	return &domain.StopTime{
		TripID:        trip.ID,
		RouteID:       trip.RouteID,
		ServiceID:     trip.ServiceID,
		Line:          line,
		Headsign:      trip.Headsign,
		ArrivalTime:   formatGTFSTime(st.ArrivalSeconds),
		DepartureTime: formatGTFSTime(st.DepartureSeconds),
		StopSequence:  int(st.StopSequence),
	}, true
}

func formatGTFSTime(totalSeconds uint32) string {
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func (s *GTFSStore) getActiveServices(dateStr string, weekday time.Weekday) map[string]bool {
	active := make(map[string]bool)

	for serviceID, cal := range s.calendars {
		if dateStr < cal.StartDate || dateStr > cal.EndDate {
			continue
		}

		isActive := false
		switch weekday {
		case time.Monday:
			isActive = cal.Monday
		case time.Tuesday:
			isActive = cal.Tuesday
		case time.Wednesday:
			isActive = cal.Wednesday
		case time.Thursday:
			isActive = cal.Thursday
		case time.Friday:
			isActive = cal.Friday
		case time.Saturday:
			isActive = cal.Saturday
		case time.Sunday:
			isActive = cal.Sunday
		}

		if isActive {
			active[serviceID] = true
		}
	}

	for serviceID, dates := range s.calendarDates {
		for _, cd := range dates {
			if cd.Date == dateStr {
				if cd.ExceptionType == 1 {
					active[serviceID] = true
				} else if cd.ExceptionType == 2 {
					delete(active, serviceID)
				}
			}
		}
	}

	return active
}

func (s *GTFSStore) GetStopLines(stopID string) []*domain.StopLine {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lines, ok := s.stopLines[stopID]
	if !ok {
		return nil
	}

	result := make([]*domain.StopLine, len(lines))
	for i, line := range lines {
		lineCopy := &domain.StopLine{
			RouteID:   line.RouteID,
			Line:      line.Line,
			LongName:  line.LongName,
			Type:      line.Type,
			Color:     line.Color,
			Headsigns: make([]string, len(line.Headsigns)),
		}
		copy(lineCopy.Headsigns, line.Headsigns)
		result[i] = lineCopy
	}
	return result
}

type GTFSStats struct {
	RoutesCount int       `json:"routes_count"`
	ShapesCount int       `json:"shapes_count"`
	StopsCount  int       `json:"stops_count"`
	LastUpdate  time.Time `json:"last_update"`
	IsLoaded    bool      `json:"is_loaded"`
}

func (s *GTFSStore) GetStats() GTFSStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return GTFSStats{
		RoutesCount: len(s.routes),
		ShapesCount: len(s.shapes),
		StopsCount:  len(s.stops),
		LastUpdate:  s.lastUpdate,
		IsLoaded:    !s.lastUpdate.IsZero(),
	}
}

func (s *GTFSStore) GetCalendarsAndDates() ([]*domain.Calendar, []*domain.CalendarDate) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	calendars := make([]*domain.Calendar, 0, len(s.calendars))
	for _, cal := range s.calendars {
		copy := *cal
		calendars = append(calendars, &copy)
	}

	var calendarDates []*domain.CalendarDate
	for _, dates := range s.calendarDates {
		for _, cd := range dates {
			copy := *cd
			calendarDates = append(calendarDates, &copy)
		}
	}

	return calendars, calendarDates
}
