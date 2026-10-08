package quartermaster

import (
	"fmt"
	"time"
)

// CampError is returned, without any network call, while the Quartermaster
// is in camp: GitHub asked us to slow down and we are waiting it out.
type CampError struct {
	Until  time.Time
	Reason string
}

func (e *CampError) Error() string {
	return fmt.Sprintf("GitHub asked us to slow down; paused until %s (%s)", e.Until.Local().Format("15:04"), e.Reason)
}

// RationError is returned when the remaining budget of a resource is below
// the floor for the request's class. Nothing is sent.
type RationError struct {
	Resource  string
	Remaining int
	Floor     int
	Reset     time.Time
}

func (e *RationError) Error() string {
	return fmt.Sprintf("GitHub request budget is low (%d left on %s, keeping %d in reserve); resets at %s",
		e.Remaining, e.Resource, e.Floor, e.Reset.Local().Format("15:04"))
}
