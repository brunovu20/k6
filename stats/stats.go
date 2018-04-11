/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package stats

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"gopkg.in/guregu/null.v3"
)

const (
	counterString = `"counter"`
	gaugeString   = `"gauge"`
	trendString   = `"trend"`
	rateString    = `"rate"`

	defaultString = `"default"`
	timeString    = `"time"`
	dataString    = `"data"`
)

// Possible values for MetricType.
const (
	Counter = MetricType(iota) // A counter that sums its data points
	Gauge                      // A gauge that displays the latest value
	Trend                      // A trend, min/max/avg/med are interesting
	Rate                       // A rate, displays % of values that aren't 0
)

// Possible values for ValueType.
const (
	Default = ValueType(iota) // Values are presented as-is
	Time                      // Values are timestamps (nanoseconds)
	Data                      // Values are data amounts (bytes)
)

// The serialized metric type is invalid.
var ErrInvalidMetricType = errors.New("Invalid metric type")

// The serialized value type is invalid.
var ErrInvalidValueType = errors.New("Invalid value type")

// A MetricType specifies the type of a metric.
type MetricType int

// MarshalJSON serializes a MetricType as a human readable string.
func (t MetricType) MarshalJSON() ([]byte, error) {
	switch t {
	case Counter:
		return []byte(counterString), nil
	case Gauge:
		return []byte(gaugeString), nil
	case Trend:
		return []byte(trendString), nil
	case Rate:
		return []byte(rateString), nil
	default:
		return nil, ErrInvalidMetricType
	}
}

// UnmarshalJSON deserializes a MetricType from a string representation.
func (t *MetricType) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case counterString:
		*t = Counter
	case gaugeString:
		*t = Gauge
	case trendString:
		*t = Trend
	case rateString:
		*t = Rate
	default:
		return ErrInvalidMetricType
	}

	return nil
}

func (t MetricType) String() string {
	switch t {
	case Counter:
		return counterString
	case Gauge:
		return gaugeString
	case Trend:
		return trendString
	case Rate:
		return rateString
	default:
		return "[INVALID]"
	}
}

// The type of values a metric contains.
type ValueType int

// MarshalJSON serializes a ValueType as a human readable string.
func (t ValueType) MarshalJSON() ([]byte, error) {
	switch t {
	case Default:
		return []byte(defaultString), nil
	case Time:
		return []byte(timeString), nil
	case Data:
		return []byte(dataString), nil
	default:
		return nil, ErrInvalidValueType
	}
}

// UnmarshalJSON deserializes a ValueType from a string representation.
func (t *ValueType) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case defaultString:
		*t = Default
	case timeString:
		*t = Time
	case dataString:
		*t = Data
	default:
		return ErrInvalidValueType
	}

	return nil
}

func (t ValueType) String() string {
	switch t {
	case Default:
		return defaultString
	case Time:
		return timeString
	case Data:
		return dataString
	default:
		return "[INVALID]"
	}
}

// SampleTags is an immutable string[string] map for tags. Once a tag
// set is created, direct modification is prohibited. It has
// copy-on-write semantics and uses pointers for faster comparison
// between maps, since the same tag set is often used for multiple samples.
// All methods should not panic, even if they are called on a nil pointer.
type SampleTags struct {
	tags map[string]string
	json []byte
}

// Get returns an empty string and false if the the requested key is not
// present or its value and true if it is.
func (st *SampleTags) Get(key string) (string, bool) {
	if st == nil {
		return "", false
	}
	val, ok := st.tags[key]
	return val, ok
}

// IsEqual tries to compare two tag sets with maximum efficiency.
func (st *SampleTags) IsEqual(other *SampleTags) bool {
	if st == other {
		return true
	}
	if st == nil || other == nil || len(st.tags) != len(other.tags) {
		return false
	}
	for k, v := range st.tags {
		if otherv, ok := other.tags[k]; !ok || v != otherv {
			return false
		}
	}
	return true
}

// MarshalJSON serializes SampleTags to a JSON string and caches
// the result. It is not thread safe in the sense that the Go race
// detector will complain if it's used concurrently, but no data
// should be corrupted.
func (st *SampleTags) MarshalJSON() ([]byte, error) {
	if st == nil {
		return []byte("null"), nil
	}
	if st.json != nil {
		return st.json, nil
	}
	res, err := json.Marshal(st.tags)
	if err != nil {
		return res, err
	}
	st.json = res
	return res, nil
}

// UnmarshalJSON deserializes SampleTags from a JSON string.
func (st *SampleTags) UnmarshalJSON(data []byte) error {
	if st == nil {
		*st = SampleTags{}
	}
	return json.Unmarshal(data, &st.tags)
}

// CloneTags copies the underlying set of a sample tags and
// returns it. If the receiver is nil, it returns an empty non-nil map.
func (st *SampleTags) CloneTags() map[string]string {
	res := map[string]string{}
	if st != nil {
		for k, v := range st.tags {
			res[k] = v
		}
	}
	return res
}

// NewSampleTags *copies* the supplied tag set and returns a new SampleTags
// instance with the key-value pairs from it.
func NewSampleTags(data map[string]string) *SampleTags {
	tags := map[string]string{}
	for k, v := range data {
		tags[k] = v
	}
	return &SampleTags{tags: tags}
}

// IntoSampleTags "consumes" the passed map and creates a new SampleTags
// struct with the data. The map is set to nil as a hint that it shouldn't
// be changed after it has been transformed into an "immutable" tag set.
// Oh, how I miss Rust and move semantics... :)
func IntoSampleTags(data *map[string]string) *SampleTags {
	res := SampleTags{tags: *data}
	*data = nil
	return &res
}

// A Sample is a single measurement.
type Sample struct {
	Metric *Metric
	Time   time.Time
	Tags   *SampleTags
	Value  float64
}

// A Metric defines the shape of a set of data.
type Metric struct {
	Name       string       `json:"name"`
	Type       MetricType   `json:"type"`
	Contains   ValueType    `json:"contains"`
	Tainted    null.Bool    `json:"tainted"`
	Thresholds Thresholds   `json:"thresholds"`
	Submetrics []*Submetric `json:"submetrics"`
	Sub        Submetric    `json:"sub,omitempty"`
	Sink       Sink         `json:"-"`
}

func New(name string, typ MetricType, t ...ValueType) *Metric {
	vt := Default
	if len(t) > 0 {
		vt = t[0]
	}
	var sink Sink
	switch typ {
	case Counter:
		sink = &CounterSink{}
	case Gauge:
		sink = &GaugeSink{}
	case Trend:
		sink = &TrendSink{}
	case Rate:
		sink = &RateSink{}
	default:
		return nil
	}
	return &Metric{Name: name, Type: typ, Contains: vt, Sink: sink}
}

func (m *Metric) HumanizeValue(v float64) string {
	switch m.Type {
	case Rate:
		// Truncate instead of round when decreasing precision to 2 decimal places
		return strconv.FormatFloat(float64(int(v*100*100))/100, 'f', 2, 64) + "%"
	default:
		switch m.Contains {
		case Time:
			d := ToD(v)
			switch {
			case d > time.Minute:
				d -= d % (1 * time.Second)
			case d > time.Second:
				d -= d % (10 * time.Millisecond)
			case d > time.Millisecond:
				d -= d % (10 * time.Microsecond)
			case d > time.Microsecond:
				d -= d % (10 * time.Nanosecond)
			}
			return d.String()
		case Data:
			return humanize.Bytes(uint64(v))
		default:
			return humanize.Ftoa(v)
		}
	}
}

// A Submetric represents a filtered dataset based on a parent metric.
type Submetric struct {
	Name   string      `json:"name"`
	Parent string      `json:"parent"`
	Suffix string      `json:"suffix"`
	Tags   *SampleTags `json:"tags"`
	Metric *Metric     `json:"-"`
}

// Creates a submetric from a name.
func NewSubmetric(name string) (parentName string, sm *Submetric) {
	parts := strings.SplitN(strings.TrimSuffix(name, "}"), "{", 2)
	if len(parts) == 1 {
		return parts[0], &Submetric{Name: name}
	}

	kvs := strings.Split(parts[1], ",")
	tags := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		if kv == "" {
			continue
		}
		parts := strings.SplitN(kv, ":", 2)

		key := strings.TrimSpace(strings.Trim(parts[0], `"'`))
		if len(parts) != 2 {
			tags[key] = ""
			continue
		}

		value := strings.TrimSpace(strings.Trim(parts[1], `"'`))
		tags[key] = value
	}
	return parts[0], &Submetric{Name: name, Parent: parts[0], Suffix: parts[1], Tags: IntoSampleTags(&tags)}
}

func (m *Metric) Summary(t time.Duration) *Summary {
	return &Summary{
		Metric:  m,
		Summary: m.Sink.Format(t),
	}
}

type Summary struct {
	Metric  *Metric            `json:"metric"`
	Summary map[string]float64 `json:"summary"`
}
