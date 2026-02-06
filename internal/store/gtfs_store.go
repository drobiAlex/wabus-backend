package store

import (
	"sync"
	"time"

	"wabus/internal/domain"
)

type GTFSStore struct {
	mu           sync.RWMutex
	routes       map[string]*domain.Route
	routesByLine map[string]*domain.Route
	shapes       map[string]*domain.Shape
	routeShapes  map[string][]string
	stops        map[string]*domain.Stop
	stopSchedules map[string][]*domain.StopTime
	stopLines     map[string][]*domain.StopLine

	lastUpdate time.Time
}

func NewGTFSStore() *GTFSStore {
	return &GTFSStore{
		routes:        make(map[string]*domain.Route),
		routesByLine:  make(map[string]*domain.Route),
		shapes:        make(map[string]*domain.Shape),
		routeShapes:   make(map[string][]string),
		stops:         make(map[string]*domain.Stop),
		stopSchedules: make(map[string][]*domain.StopTime),
		stopLines:     make(map[string][]*domain.StopLine),
	}
}

func (s *GTFSStore) UpdateAll(routes map[string]*domain.Route, shapes map[string]*domain.Shape, stops map[string]*domain.Stop, routeShapes map[string][]string, stopSchedules map[string][]*domain.StopTime, stopLines map[string][]*domain.StopLine) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.routes = routes
	s.shapes = shapes
	s.stops = stops
	s.routeShapes = routeShapes
	s.stopSchedules = stopSchedules
	s.stopLines = stopLines
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

	shapeIDs, ok := s.routeShapes[routeID]
	if !ok {
		return nil
	}

	result := make([]*domain.Shape, 0, len(shapeIDs))
	for _, shapeID := range shapeIDs {
		if shape, ok := s.shapes[shapeID]; ok {
			shapeCopy := &domain.Shape{
				ID:     shape.ID,
				Points: make([]domain.ShapePoint, len(shape.Points)),
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

func (s *GTFSStore) GetStopSchedule(stopID string) []*domain.StopTime {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schedule, ok := s.stopSchedules[stopID]
	if !ok {
		return nil
	}

	result := make([]*domain.StopTime, len(schedule))
	for i, st := range schedule {
		copy := *st
		result[i] = &copy
	}
	return result
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
