package envoy_access_log_server

import (
	"fmt"

	"github.com/golang/protobuf/ptypes"
	pbduration "github.com/golang/protobuf/ptypes/duration"
)

// Returns deterministic string representation of 'pbduration' - ns
func pbDurationDeterministicString(pbduration *pbduration.Duration) (string, error) {
	if pbduration == nil {
		return "", fmt.Errorf("<nil> duration given")
	}
	duration, err := ptypes.Duration(pbduration)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(duration.Nanoseconds()) + "ns", nil
}
