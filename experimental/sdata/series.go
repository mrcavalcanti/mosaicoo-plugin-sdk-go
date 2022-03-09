package sdata

import (
	"sort"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// Less sure about this interface.
// Also, probably would need to make a type with methods that wrap the interface so methods can be added
// without breaking changes - if we even want an interface like this
// But for now helps illustrate at least
type TimeSeriesCollectionWriter interface {
	AddMetric(metricName string, l data.Labels, t []time.Time, values interface{}) error
	SetMetricMD(metricName string, l data.Labels, fc data.FieldConfig)
	// AddField Possible TODO - If we accept certain extra information to be valid but outside the series
}

type TimeSeriesCollection interface {
	TimeSeriesCollectionReader
	TimeSeriesCollectionWriter
}

type TimeSeriesCollectionReader interface {
	Validate() (isEmpty bool, ignoredFieldIndices []FrameFieldIndex, errors []error)
	GetMetricRefs() ([]TimeSeriesMetricRef, []FrameFieldIndex)
}

func ValidValueFields() []data.FieldType {
	return append(data.NumericFieldTypes(), []data.FieldType{data.FieldTypeBool, data.FieldTypeNullableBool}...)
}

// I am not sure about this but want to get the idea down
type TimeSeriesMetricRef struct {
	ValueField *data.Field
	TimeField  *data.Field
}

func (m TimeSeriesMetricRef) GetMetricName() string {
	if m.ValueField != nil {
		return m.ValueField.Name
	}
	return ""
}

// TODO GetFQMetric (or something, Names + Labels)

func (m TimeSeriesMetricRef) GetLabels() data.Labels {
	if m.ValueField != nil {
		return m.ValueField.Labels
	}
	return nil
}

type FrameFieldIndex struct {
	FrameIdx int // -1 means nil Frame
	FieldIdx int // -1 means no fields
}

type FrameFieldIndices []FrameFieldIndex

func (f FrameFieldIndices) Len() int {
	return len(f)
}

func (f FrameFieldIndices) Less(i, j int) bool {
	if f[i].FrameIdx == f[j].FrameIdx {
		return f[i].FieldIdx < f[j].FieldIdx
	}
	return f[i].FrameIdx < f[j].FrameIdx
}

func (f FrameFieldIndices) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

func sortTimeSeriesMetricRef(refs []TimeSeriesMetricRef) {
	sort.SliceStable(refs, func(i, j int) bool {
		iRef := refs[i]
		jRef := refs[j]

		if iRef.GetMetricName() < jRef.GetMetricName() {
			return true
		}
		if iRef.GetMetricName() > jRef.GetMetricName() {
			return false
		}

		// If here Names are equal, next sort based on if there are labels.

		if iRef.GetLabels() == nil && jRef.GetLabels() == nil {
			return true // no labels first
		}
		if iRef.GetLabels() == nil && jRef.GetLabels() != nil {
			return true
		}
		if iRef.GetLabels() != nil && jRef.GetLabels() == nil {
			return false
		}

		return iRef.GetLabels().String() < jRef.GetLabels().String()
	})
}