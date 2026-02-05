package domain

import "time"

// VehicleType distinguishes buses from trams
type VehicleType int

const (
	VehicleTypeBus  VehicleType = 1
	VehicleTypeTram VehicleType = 2
)

func (t VehicleType) String() string {
	switch t {
	case VehicleTypeBus:
		return "bus"
	case VehicleTypeTram:
		return "tram"
	default:
		return "unknown"
	}
}

// Vehicle represents a single bus or tram position
type Vehicle struct {
	Key           string      `json:"key"`
	VehicleNumber string      `json:"vehicleNumber"`
	Type          VehicleType `json:"type"`
	Line          string      `json:"line"`
	Brigade       string      `json:"brigade"`
	Lat           float64     `json:"lat"`
	Lon           float64     `json:"lon"`
	Timestamp     time.Time   `json:"timestamp"`
	TileID        string      `json:"tileId"`
	UpdatedAt     time.Time   `json:"updatedAt"`
}

// DeltaType indicates whether a vehicle was updated or removed
type DeltaType string

const (
	DeltaUpdate DeltaType = "update"
	DeltaRemove DeltaType = "remove"
)

// VehicleDelta represents a change in vehicle state
type VehicleDelta struct {
	Type    DeltaType `json:"type"`
	Vehicle *Vehicle  `json:"vehicle,omitempty"`
	Key     string    `json:"key,omitempty"`
	TileID  string    `json:"tileId"`
}

// BoundingBox represents a geographic rectangle
type BoundingBox struct {
	MinLat float64 `json:"minLat"`
	MaxLat float64 `json:"maxLat"`
	MinLon float64 `json:"minLon"`
	MaxLon float64 `json:"maxLon"`
}

// Contains checks if a point is within the bounding box
func (bb *BoundingBox) Contains(lat, lon float64) bool {
	return lat >= bb.MinLat && lat <= bb.MaxLat &&
		lon >= bb.MinLon && lon <= bb.MaxLon
}
