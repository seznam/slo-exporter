package envoy_access_log_server

import (
	"fmt"

	pbduration "github.com/golang/protobuf/ptypes/duration"
)

// Returns deterministic string representation of 'pbduration' - ns.
func pbDurationDeterministicString(d *pbduration.Duration) (string, error) {
	if d == nil {
		return "", fmt.Errorf("<nil> duration given")
	}
	if !d.IsValid() {
		return "", fmt.Errorf("invalid duration given: %s", d)
	}
	return fmt.Sprint(d.AsDuration().Nanoseconds()) + "ns", nil
}
