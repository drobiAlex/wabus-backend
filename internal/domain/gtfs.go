package domain

// RouteType distinguishes transport types in GTFS
type RouteType int

const (
	RouteTypeTram       RouteType = 0
	RouteTypeSubway     RouteType = 1
	RouteTypeRail       RouteType = 2
	RouteTypeBus        RouteType = 3
	RouteTypeFerry      RouteType = 4
	RouteTypeCableTram  RouteType = 5
	RouteTypeAerialLift RouteType = 6
	RouteTypeFunicular  RouteType = 7
)

func (t RouteType) String() string {
	switch t {
	case RouteTypeTram:
		return "tram"
	case RouteTypeSubway:
		return "subway"
	case RouteTypeRail:
		return "rail"
	case RouteTypeBus:
		return "bus"
	case RouteTypeFerry:
		return "ferry"
	case RouteTypeCableTram:
		return "cable_tram"
	case RouteTypeAerialLift:
		return "aerial_lift"
	case RouteTypeFunicular:
		return "funicular"
	default:
		return "unknown"
	}
}

// Route represents a transit route from GTFS
type Route struct {
	ID        string    `json:"id"`
	ShortName string    `json:"short_name"`
	LongName  string    `json:"long_name"`
	Type      RouteType `json:"type"`
	Color     string    `json:"color"`
	TextColor string    `json:"text_color"`
}

// ShapePoint represents a single point in a route shape
type ShapePoint struct {
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Sequence int     `json:"sequence"`
}

// Shape represents the geographic path of a route
type Shape struct {
	ID     string       `json:"id"`
	Points []ShapePoint `json:"points"`
}

// Stop represents a transit stop from GTFS
type Stop struct {
	ID   string  `json:"id"`
	Code string  `json:"code"`
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	Zone string  `json:"zone"`
}
