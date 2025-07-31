package helper

import "time"

// TimingCollector collects timing measurements during benchmark operations
// for detailed performance analysis.
type TimingCollector struct {
	QueryTime     *time.Duration
	UnmarshalTime *time.Duration
	BusinessTime  *time.Duration
	AppendTime    *time.Duration
}

// NewTimingCollector creates a new TimingCollector with
// pointers to duration variables that will accumulate timing measurements.
func NewTimingCollector(queryTime, unmarshalTime, businessTime, appendTime *time.Duration) TimingCollector {
	return TimingCollector{
		QueryTime:     queryTime,
		UnmarshalTime: unmarshalTime,
		BusinessTime:  businessTime,
		AppendTime:    appendTime,
	}
}

func (t TimingCollector) RecordQuery(duration time.Duration) {
	if t.QueryTime != nil {
		*t.QueryTime += duration
	}
}

func (t TimingCollector) RecordUnmarshal(duration time.Duration) {
	if t.UnmarshalTime != nil {
		*t.UnmarshalTime += duration
	}
}

func (t TimingCollector) RecordBusiness(duration time.Duration) {
	if t.BusinessTime != nil {
		*t.BusinessTime += duration
	}
}

func (t TimingCollector) RecordAppend(duration time.Duration) {
	if t.AppendTime != nil {
		*t.AppendTime += duration
	}
}