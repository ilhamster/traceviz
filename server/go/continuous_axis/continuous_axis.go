/*
	Copyright 2023 Google Inc.
	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at
		https://www.apache.org/licenses/LICENSE-2.0
	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/

// Package continuousaxis provides decorator helpers for defining continuous
// axes.  An axis has a name, a label, a type which describes that axis'
// domain, and minimum and maximum points along that domain.
package continuousaxis

import (
	"fmt"
	"math"
	"time"

	"github.com/ilhamster/traceviz/server/go/category"
	"github.com/ilhamster/traceviz/server/go/util"
)

const (
	axisTypeKey = "axis_type"
	axisMinKey  = "axis_min"
	axisMaxKey  = "axis_max"

	timestampAxisType = "timestamp"
	durationAxisType  = "duration"
	doubleAxisType    = "double"

	xAxisRenderLabelHeightPxKey   = "x_axis_render_label_height_px"
	xAxisRenderMarkersHeightPxKey = "x_axis_render_markers_height_px"
	yAxisRenderLabelHeightPxKey   = "y_axis_render_label_width_px"
	yAxisRenderMarkersHeightPxKey = "y_axis_render_markers_width_px"
)

// XAxisRenderSettings contains configuring an X axis.
type XAxisRenderSettings struct {
	LabelHeightPx   int64
	MarkersHeightPx int64
}

// Apply annotates with the receiving XAxisRenderSettings.
func (x XAxisRenderSettings) Apply() util.PropertyUpdate {
	return util.Chain(
		util.IntegerProperty(xAxisRenderLabelHeightPxKey, x.LabelHeightPx),
		util.IntegerProperty(xAxisRenderMarkersHeightPxKey, x.MarkersHeightPx),
	)
}

// YAxisRenderSettings contains configuring a Y axis.
type YAxisRenderSettings struct {
	LabelWidthPx   int64
	MarkersWidthPx int64
}

// Apply annotates with the receiving YAxisRenderSettings.
func (y YAxisRenderSettings) Apply() util.PropertyUpdate {
	return util.Chain(
		util.IntegerProperty(yAxisRenderLabelHeightPxKey, y.LabelWidthPx),
		util.IntegerProperty(yAxisRenderMarkersHeightPxKey, y.MarkersWidthPx),
	)
}

// Axis is implemented by types that can act as axes.
type axis[T float64 | time.Duration | time.Time] struct {
	axisType string
	cat      *category.Category
	value    func(key string, v T) util.PropertyUpdate
	min, max T
}

func newAxis[T float64 | time.Duration | time.Time](
	axisType string,
	cat *category.Category,
	valueFn func(key string, v T) util.PropertyUpdate,
	min, max T) *axis[T] {
	return &axis[T]{
		axisType: axisType,
		cat:      cat,
		value:    valueFn,
		min:      min,
		max:      max,
	}
}

// Define annotates with a definition of the receiver.
func (a *axis[T]) Define() util.PropertyUpdate {
	return util.Chain(
		a.cat.Define(),
		util.StringProperty(axisTypeKey, a.axisType),
		a.value(axisMinKey, a.min),
		a.value(axisMaxKey, a.max),
	)
}

func (a *axis[T]) Value(k string, v T) util.PropertyUpdate {
	return a.value(k, v)
}

// CategoryID returns the category ID of the receiving Axis.
func (a *axis[T]) CategoryID() string {
	return a.cat.ID()
}

type Axis[T any] interface {
	Define() util.PropertyUpdate
	CategoryID() string
	Value(k string, v T) util.PropertyUpdate
}

// NewTimestampAxis returns a new TimestampAxis with the specified category.
// If the optional extents are provided, the axis' minimum and maximum extents
// will be initialized to the lowest and highest of those extents.
func NewTimestampAxis(cat *category.Category, extents ...time.Time) Axis[time.Time] {
	var min, max time.Time
	for _, extent := range extents {
		if min.IsZero() || min.After(extent) {
			min = extent
		}
		if max.IsZero() || max.Before(extent) {
			max = extent
		}
	}
	return newAxis[time.Time](
		timestampAxisType, cat,
		func(key string, v time.Time) util.PropertyUpdate {
			return util.TimestampProperty(key, v)
		}, min, max)
}

// NewDurationAxis returns a new DurationAxis with the specified category.
// If the optional extents are provided, the axis' minimum and maximum extents
// will be initialized to the lowest and highest of those extents.
func NewDurationAxis(cat *category.Category, extents ...time.Duration) Axis[time.Duration] {
	var min, max time.Duration = time.Duration(math.MaxInt64), time.Duration(math.MinInt64)
	for _, extent := range extents {
		if extent < min {
			min = extent
		}
		if extent > max {
			max = extent
		}
	}
	return newAxis[time.Duration](
		durationAxisType, cat,
		func(key string, v time.Duration) util.PropertyUpdate {
			return util.DurationProperty(key, v)
		}, min, max)
}

// NewDoubleAxis returns a new DoubleAxis with the specified category.
// If the optional extents are provided, the axis' minimum and maximum extents
// will be initialized to the lowest and highest of those extents.
func NewDoubleAxis(cat *category.Category, extents ...float64) Axis[float64] {
	var min, max float64 = math.MaxFloat64, -math.MaxFloat64
	for _, extent := range extents {
		if min > extent {
			min = extent
		}
		if max < extent {
			max = extent
		}
	}
	return newAxis[float64](
		doubleAxisType, cat,
		func(key string, v float64) util.PropertyUpdate {
			return util.DoubleProperty(key, v)
		}, min, max)
}

func NewAxis[T float64 | time.Duration | time.Time](
	cat *category.Category,
	min, max T,
) (Axis[T], error) {
	switch v := any(min).(type) {
	case float64:
		axis := NewDoubleAxis(cat, v, any(max).(float64))
		return any(axis).(Axis[T]), nil
	case time.Duration:
		axis := NewDurationAxis(cat, v, any(max).(time.Duration))
		return any(axis).(Axis[T]), nil
	case time.Time:
		axis := NewTimestampAxis(cat, v, any(max).(time.Time))
		return any(axis).(Axis[T]), nil
	default:
		return nil, fmt.Errorf("unsupported type %T for axis", v)
	}
}
