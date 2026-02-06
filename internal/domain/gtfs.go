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

// StopTime represents a scheduled arrival at a stop
type StopTime struct {
	TripID        string `json:"trip_id"`
	RouteID       string `json:"route_id"`
	ServiceID     string `json:"-"` // Used for filtering, not exposed in API
	Line          string `json:"line"`
	Headsign      string `json:"headsign"`
	ArrivalTime   string `json:"arrival_time"`
	DepartureTime string `json:"departure_time"`
	StopSequence  int    `json:"stop_sequence"`
}

// Calendar represents service availability by day of week
type Calendar struct {
	ServiceID string
	Monday    bool
	Tuesday   bool
	Wednesday bool
	Thursday  bool
	Friday    bool
	Saturday  bool
	Sunday    bool
	StartDate string // YYYYMMDD
	EndDate   string // YYYYMMDD
}

// CalendarDate represents service exceptions
type CalendarDate struct {
	ServiceID     string
	Date          string // YYYYMMDD
	ExceptionType int    // 1 = added, 2 = removed
}

// StopLine represents a line that serves a stop
type StopLine struct {
	RouteID   string    `json:"route_id"`
	Line      string    `json:"line"`
	LongName  string    `json:"long_name"`
	Type      RouteType `json:"type"`
	Color     string    `json:"color"`
	Headsigns []string  `json:"headsigns"`
}
