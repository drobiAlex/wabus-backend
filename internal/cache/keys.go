package cache

import "fmt"

const (
	KeySyncFull         = "sync:full"
	KeyRoutes           = "routes"
	KeyStops            = "stops"
	KeyCalendars        = "calendars"
	KeyCalendarDates    = "calendar_dates"
	KeyGTFSVersion      = "gtfs:version"
)

func KeyScheduleToday(stopID string) string {
	return fmt.Sprintf("schedule:today:%s", stopID)
}

func KeyScheduleTomorrow(stopID string) string {
	return fmt.Sprintf("schedule:tomorrow:%s", stopID)
}

func KeyStopLines(stopID string) string {
	return fmt.Sprintf("lines:%s", stopID)
}

func KeyRouteShape(routeID string) string {
	return fmt.Sprintf("shape:%s", routeID)
}
