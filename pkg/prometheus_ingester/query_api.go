package prometheus_ingester

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	json "github.com/json-iterator/go"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type apiResponse struct {
	Status    string          `json:"status"`
	Data      json.RawMessage `json:"data"`
	ErrorType v1.ErrorType    `json:"errorType"`
	Error     string          `json:"error"`
	Warnings  []string        `json:"warnings,omitempty"`
}

// queryResult contains result data for a query.
type queryResultPrometheus struct {
	Type   model.ValueType `json:"resultType"`
	Result interface{}     `json:"result"`

	// The decoded value.``
	v model.Value
}

func (qr *queryResultPrometheus) UnmarshalJSON(b []byte) error {
	v := struct {
		Type   model.ValueType `json:"resultType"`
		Result json.RawMessage `json:"result"`
	}{}

	err := json.Unmarshal(b, &v)
	if err != nil {
		return err
	}

	switch v.Type {
	case model.ValScalar:
		var sv model.Scalar
		err = json.Unmarshal(v.Result, &sv)
		qr.v = &sv

	case model.ValVector:
		var vv model.Vector
		err = json.Unmarshal(v.Result, &vv)
		qr.v = vv

	case model.ValMatrix:
		var mv model.Matrix
		err = json.Unmarshal(v.Result, &mv)
		qr.v = mv

	default:
		err = fmt.Errorf("unexpected value type %q", v.Type)
	}
	return err
}

func (q *queryExecutor) queryPrometheus(ctx context.Context, queryPromQL string, ts time.Time) (model.Value, v1.Warnings, error) {
	u := q.client.URL("api/v1/query", nil)
	params := u.Query()
	params.Add("query", queryPromQL)
	// TODO(FUSAKLA) this should be configurable
	params.Add("dedup", "false")
	params.Add("time", strconv.FormatFloat(float64(ts.Unix())+float64(ts.Nanosecond())/1e9, 'f', -1, 64))
	u.RawQuery = params.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, nil, err
	}

	resp, body, err := q.client.Do(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, nil, fmt.Errorf("unexpected status code %d, error: %s", resp.StatusCode, body)
	}

	var result apiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, nil, err
	}

	if result.Status == "error" {
		return nil, nil, fmt.Errorf(result.Error)
	}

	var qres queryResultPrometheus
	if err := json.Unmarshal(result.Data, &qres); err != nil {
		return nil, nil, err
	}
	return model.Value(qres.v), result.Warnings, nil
}
