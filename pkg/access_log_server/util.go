package access_log_server

import (
	"fmt"
	"time"

	pbduration "github.com/golang/protobuf/ptypes/duration"
	"github.com/pkg/errors"
)

// Returns deterministic string representation of 'pbduration' - ns
// panics in case of error
func pbDurationDeterministicString(pbduration *pbduration.Duration) string {
	if pbduration == nil {
		return "<nil>"
	}
	duration, err := time.ParseDuration(fmt.Sprintf("%ds%dns", pbduration.Seconds, pbduration.Nanos))
	if err != nil {
		panic(errors.Wrap(err, "Unable to convert protobuf Duration to stdlib Duration"))
	}
	return fmt.Sprint(duration.Nanoseconds())
}
