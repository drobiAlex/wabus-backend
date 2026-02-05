package store

import (
	"sync"
	"time"

	"wabus/internal/domain"
)

type ListOptions struct {
	Type *domain.VehicleType
	Line string
	BBox *domain.BoundingBox
}

type Store struct {
	mu       sync.RWMutex
	vehicles map[string]*domain.Vehicle
	byTile   map[string]map[string]struct{}
	byLine   map[string]map[string]struct{}
	byType   map[domain.VehicleType]map[string]struct{}

	staleAfter time.Duration
}

func New(staleAfter time.Duration) *Store {
	return &Store{
		vehicles:   make(map[string]*domain.Vehicle),
		byTile:     make(map[string]map[string]struct{}),
		byLine:     make(map[string]map[string]struct{}),
		byType:     make(map[domain.VehicleType]map[string]struct{}),
		staleAfter: staleAfter,
	}
}

func (s *Store) Update(vehicles []*domain.Vehicle) []domain.VehicleDelta {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	deltas := make([]domain.VehicleDelta, 0, len(vehicles))

	for _, v := range vehicles {
		v.UpdatedAt = now

		existing, exists := s.vehicles[v.Key]
		if !exists || hasChanged(existing, v) {
			if exists && existing.TileID != v.TileID {
				s.removeFromTileIndex(existing.Key, existing.TileID)
			}

			s.vehicles[v.Key] = v
			s.addToIndices(v)

			deltas = append(deltas, domain.VehicleDelta{
				Type:    domain.DeltaUpdate,
				Vehicle: v,
				TileID:  v.TileID,
			})
		} else {
			existing.UpdatedAt = now
		}
	}

	return deltas
}

func (s *Store) PruneStale() []domain.VehicleDelta {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-s.staleAfter)
	var deltas []domain.VehicleDelta

	for key, v := range s.vehicles {
		if v.UpdatedAt.Before(cutoff) {
			deltas = append(deltas, domain.VehicleDelta{
				Type:   domain.DeltaRemove,
				Key:    key,
				TileID: v.TileID,
			})
			s.removeFromAllIndices(v)
			delete(s.vehicles, key)
		}
	}

	return deltas
}

func (s *Store) Get(key string) (*domain.Vehicle, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.vehicles[key]
	if !ok {
		return nil, false
	}
	copy := *v
	return &copy, true
}

func (s *Store) List(opts ListOptions) []*domain.Vehicle {
	s.mu.RLock()
	defer s.mu.RUnlock()

	candidates := s.getCandidates(opts)

	result := make([]*domain.Vehicle, 0, len(candidates))
	for key := range candidates {
		v := s.vehicles[key]
		if opts.BBox != nil && !opts.BBox.Contains(v.Lat, v.Lon) {
			continue
		}
		copy := *v
		result = append(result, &copy)
	}

	return result
}

func (s *Store) Snapshot() []*domain.Vehicle {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*domain.Vehicle, 0, len(s.vehicles))
	for _, v := range s.vehicles {
		copy := *v
		result = append(result, &copy)
	}
	return result
}

func (s *Store) SnapshotForTiles(tileIDs []string) []*domain.Vehicle {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := make(map[string]struct{})
	var result []*domain.Vehicle

	for _, tileID := range tileIDs {
		if keys, ok := s.byTile[tileID]; ok {
			for key := range keys {
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				v := s.vehicles[key]
				copy := *v
				result = append(result, &copy)
			}
		}
	}
	return result
}

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.vehicles)
}

func (s *Store) getCandidates(opts ListOptions) map[string]struct{} {
	if opts.Type != nil && opts.Line != "" {
		return s.intersect(s.byType[*opts.Type], s.byLine[opts.Line])
	}
	if opts.Type != nil {
		return s.copySet(s.byType[*opts.Type])
	}
	if opts.Line != "" {
		return s.copySet(s.byLine[opts.Line])
	}

	result := make(map[string]struct{}, len(s.vehicles))
	for key := range s.vehicles {
		result[key] = struct{}{}
	}
	return result
}

func (s *Store) intersect(a, b map[string]struct{}) map[string]struct{} {
	if a == nil || b == nil {
		return make(map[string]struct{})
	}

	smaller, larger := a, b
	if len(a) > len(b) {
		smaller, larger = b, a
	}

	result := make(map[string]struct{})
	for key := range smaller {
		if _, ok := larger[key]; ok {
			result[key] = struct{}{}
		}
	}
	return result
}

func (s *Store) copySet(src map[string]struct{}) map[string]struct{} {
	if src == nil {
		return make(map[string]struct{})
	}
	result := make(map[string]struct{}, len(src))
	for key := range src {
		result[key] = struct{}{}
	}
	return result
}

func (s *Store) addToIndices(v *domain.Vehicle) {
	if s.byTile[v.TileID] == nil {
		s.byTile[v.TileID] = make(map[string]struct{})
	}
	s.byTile[v.TileID][v.Key] = struct{}{}

	if s.byLine[v.Line] == nil {
		s.byLine[v.Line] = make(map[string]struct{})
	}
	s.byLine[v.Line][v.Key] = struct{}{}

	if s.byType[v.Type] == nil {
		s.byType[v.Type] = make(map[string]struct{})
	}
	s.byType[v.Type][v.Key] = struct{}{}
}

func (s *Store) removeFromTileIndex(key, tileID string) {
	if s.byTile[tileID] != nil {
		delete(s.byTile[tileID], key)
		if len(s.byTile[tileID]) == 0 {
			delete(s.byTile, tileID)
		}
	}
}

func (s *Store) removeFromAllIndices(v *domain.Vehicle) {
	s.removeFromTileIndex(v.Key, v.TileID)

	if s.byLine[v.Line] != nil {
		delete(s.byLine[v.Line], v.Key)
		if len(s.byLine[v.Line]) == 0 {
			delete(s.byLine, v.Line)
		}
	}

	if s.byType[v.Type] != nil {
		delete(s.byType[v.Type], v.Key)
		if len(s.byType[v.Type]) == 0 {
			delete(s.byType, v.Type)
		}
	}
}

func hasChanged(old, new *domain.Vehicle) bool {
	const epsilon = 0.000001

	if old.Line != new.Line || old.Brigade != new.Brigade {
		return true
	}

	latDiff := old.Lat - new.Lat
	if latDiff < 0 {
		latDiff = -latDiff
	}
	lonDiff := old.Lon - new.Lon
	if lonDiff < 0 {
		lonDiff = -lonDiff
	}

	if latDiff > epsilon || lonDiff > epsilon {
		return true
	}

	if !old.Timestamp.Equal(new.Timestamp) {
		return true
	}

	return false
}
