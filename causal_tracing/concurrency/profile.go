// Package concurrency provides reusable activity-profile data structures for
// rendering concurrency heatmaps.
package concurrency

import (
	"fmt"
	"math"
	"sort"

	"github.com/ilhamster/tracey/trace"
)

const defaultBucketWidthPx = 4.0

// Range is a half-open interval in Tracey comparator difference units.
type Range struct {
	Start float64
	End   float64
}

// Interval is one weighted contribution to a concurrency profile. Weight is
// usually one for a single unsuspended span interval.
type Interval struct {
	Start  float64
	End    float64
	Weight int
}

// Profile stores a piecewise-constant concurrency count over time.
type Profile struct {
	ID       string
	Start    float64
	End      float64
	Peak     int
	Segments []Segment
}

// Segment is one interval of constant concurrency count.
type Segment struct {
	Start float64
	End   float64
	Count int
}

// Bucket is a view-specific downsampled portion of a Profile.
type Bucket struct {
	Start float64
	End   float64
	Avg   float64
	Peak  int
}

// BucketOptions controls how a Profile is downsampled for one rendered view.
type BucketOptions struct {
	// Domain is the full temporal domain represented by the rendered trace
	// view. Bucket boundaries are computed across this whole domain so adjacent
	// rendered spans align on pixel boundaries.
	Domain Range
	// Clip restricts the returned buckets to one rendered span or visible
	// interval. If empty, the profile is clipped only to Domain.
	Clip Range
	// WidthPx is the trace view width in CSS pixels.
	WidthPx int
	// MinimumFeatureWidthPx is the desired minimum bucket width in pixels. It is
	// clamped to a package default to avoid over-rendering fine-grained buckets.
	MinimumFeatureWidthPx float64
}

type endpoint struct {
	moment float64
	delta  int
}

// FromIntervals builds a Profile by summing weighted intervals.
func FromIntervals(id string, intervals []Interval) *Profile {
	endpoints := make([]endpoint, 0, 2*len(intervals))
	for _, interval := range intervals {
		if interval.End <= interval.Start || interval.Weight <= 0 {
			continue
		}
		endpoints = append(endpoints,
			endpoint{moment: interval.Start, delta: interval.Weight},
			endpoint{moment: interval.End, delta: -interval.Weight},
		)
	}
	return fromEndpoints(id, endpoints)
}

// Merge builds a Profile by summing existing profiles. This is useful for
// bottom-up category aggregation.
func Merge(id string, profiles ...*Profile) *Profile {
	var intervals []Interval
	for _, profile := range profiles {
		if profile == nil {
			continue
		}
		for _, segment := range profile.Segments {
			intervals = append(intervals, Interval{
				Start:  segment.Start,
				End:    segment.End,
				Weight: segment.Count,
			})
		}
	}
	return FromIntervals(id, intervals)
}

// ProfilesByTraceCategories builds one Profile for each category in the
// requested hierarchy types. If hierarchyTypes is empty, it uses all hierarchy
// types observed by the trace.
func ProfilesByTraceCategories[T any, CP, SP, DP fmt.Stringer](
	tr trace.Trace[T, CP, SP, DP],
	origin T,
	hierarchyTypes ...trace.HierarchyType,
) map[string]*Profile {
	ret := map[string]*Profile{}
	if tr == nil {
		return ret
	}
	if len(hierarchyTypes) == 0 {
		hierarchyTypes = tr.HierarchyTypes()
	}
	namer := tr.DefaultNamer()
	seenCategoryIDs := map[string]struct{}{}
	for _, hierarchyType := range hierarchyTypes {
		for _, rootCategory := range tr.RootCategories(hierarchyType) {
			appendTraceCategoryProfiles(ret, seenCategoryIDs, tr.Comparator(), namer, origin, rootCategory)
		}
	}
	return ret
}

// FromTraceCategory builds a Profile for a Tracey category subtree. The
// profile coordinates are comparator differences from origin.
func FromTraceCategory[T any, CP, SP, DP fmt.Stringer](
	category trace.Category[T, CP, SP, DP],
	comparator trace.Comparator[T],
	namer trace.Namer[T, CP, SP, DP],
	origin T,
) *Profile {
	intervals := []Interval{}
	seenSpanIDs := map[string]struct{}{}
	appendTraceCategoryIntervals(&intervals, seenSpanIDs, comparator, namer, origin, category)
	return FromIntervals(namer.CategoryUniqueID(category), intervals)
}

func appendTraceCategoryProfiles[T any, CP, SP, DP fmt.Stringer](
	profiles map[string]*Profile,
	seenCategoryIDs map[string]struct{},
	comparator trace.Comparator[T],
	namer trace.Namer[T, CP, SP, DP],
	origin T,
	category trace.Category[T, CP, SP, DP],
) {
	categoryID := namer.CategoryUniqueID(category)
	if _, ok := seenCategoryIDs[categoryID]; !ok {
		seenCategoryIDs[categoryID] = struct{}{}
		profiles[categoryID] = FromTraceCategory(category, comparator, namer, origin)
	}
	for _, child := range category.ChildCategories() {
		appendTraceCategoryProfiles(profiles, seenCategoryIDs, comparator, namer, origin, child)
	}
}

func appendTraceCategoryIntervals[T any, CP, SP, DP fmt.Stringer](
	intervals *[]Interval,
	seenSpanIDs map[string]struct{},
	comparator trace.Comparator[T],
	namer trace.Namer[T, CP, SP, DP],
	origin T,
	category trace.Category[T, CP, SP, DP],
) {
	for _, rootSpan := range category.RootSpans() {
		appendTraceSpanIntervals(intervals, seenSpanIDs, comparator, namer, origin, rootSpan)
	}
	for _, child := range category.ChildCategories() {
		appendTraceCategoryIntervals(intervals, seenSpanIDs, comparator, namer, origin, child)
	}
}

func appendTraceSpanIntervals[T any, CP, SP, DP fmt.Stringer](
	intervals *[]Interval,
	seenSpanIDs map[string]struct{},
	comparator trace.Comparator[T],
	namer trace.Namer[T, CP, SP, DP],
	origin T,
	span trace.Span[T, CP, SP, DP],
) {
	spanID := namer.SpanUniqueID(span)
	if _, ok := seenSpanIDs[spanID]; ok {
		return
	}
	seenSpanIDs[spanID] = struct{}{}
	for _, elementarySpan := range span.ElementarySpans() {
		if !comparator.Greater(elementarySpan.End(), elementarySpan.Start()) {
			continue
		}
		*intervals = append(*intervals, Interval{
			Start:  comparator.Diff(elementarySpan.Start(), origin),
			End:    comparator.Diff(elementarySpan.End(), origin),
			Weight: 1,
		})
	}
	for _, child := range span.ChildSpans() {
		appendTraceSpanIntervals(intervals, seenSpanIDs, comparator, namer, origin, child)
	}
}

func fromEndpoints(id string, endpoints []endpoint) *Profile {
	if len(endpoints) == 0 {
		return &Profile{ID: id}
	}
	sort.SliceStable(endpoints, func(i, j int) bool {
		if endpoints[i].moment != endpoints[j].moment {
			return endpoints[i].moment < endpoints[j].moment
		}
		return endpoints[i].delta < endpoints[j].delta
	})
	ret := &Profile{ID: id}
	current := 0
	lastMoment := endpoints[0].moment
	for idx := 0; idx < len(endpoints); {
		moment := endpoints[idx].moment
		if moment > lastMoment && current > 0 {
			ret.Segments = append(ret.Segments, Segment{
				Start: lastMoment,
				End:   moment,
				Count: current,
			})
			if current > ret.Peak {
				ret.Peak = current
			}
		}
		for idx < len(endpoints) && endpoints[idx].moment == moment {
			current += endpoints[idx].delta
			idx++
		}
		lastMoment = moment
	}
	if len(ret.Segments) > 0 {
		ret.Start = ret.Segments[0].Start
		ret.End = ret.Segments[len(ret.Segments)-1].End
	}
	return ret
}

// Buckets downsamples the profile for a particular rendered view.
func (p *Profile) Buckets(opts BucketOptions) []Bucket {
	if p == nil || len(p.Segments) == 0 {
		return nil
	}
	if opts.Domain.End <= opts.Domain.Start || opts.WidthPx <= 0 {
		return nil
	}
	minFeatureWidthPx := opts.MinimumFeatureWidthPx
	if minFeatureWidthPx < defaultBucketWidthPx {
		minFeatureWidthPx = defaultBucketWidthPx
	}
	bucketCount := int(math.Ceil(float64(opts.WidthPx) / minFeatureWidthPx))
	if bucketCount <= 0 {
		return nil
	}
	clip := opts.Clip
	if clip.End <= clip.Start {
		clip = opts.Domain
	}
	weightedConcurrency := make([]float64, bucketCount)
	peakConcurrency := make([]int, bucketCount)
	domainDuration := opts.Domain.End - opts.Domain.Start
	for _, segment := range p.Segments {
		segmentStart := math.Max(segment.Start, math.Max(opts.Domain.Start, clip.Start))
		segmentEnd := math.Min(segment.End, math.Min(opts.Domain.End, clip.End))
		if segmentEnd <= segmentStart {
			continue
		}
		startIdx := bucketIndexFloor(segmentStart, opts.Domain, bucketCount)
		endIdx := bucketIndexCeil(segmentEnd, opts.Domain, bucketCount) - 1
		if startIdx < 0 {
			startIdx = 0
		}
		if endIdx >= bucketCount {
			endIdx = bucketCount - 1
		}
		for idx := startIdx; idx <= endIdx; idx++ {
			bucketStart, bucketEnd := bucketRange(opts.Domain, domainDuration, bucketCount, idx)
			overlapStart := math.Max(segmentStart, bucketStart)
			overlapEnd := math.Min(segmentEnd, bucketEnd)
			if overlapEnd <= overlapStart {
				continue
			}
			weightedConcurrency[idx] += float64(segment.Count) * (overlapEnd - overlapStart)
			if segment.Count > peakConcurrency[idx] {
				peakConcurrency[idx] = segment.Count
			}
		}
	}
	var ret []Bucket
	for idx := 0; idx < bucketCount; idx++ {
		if weightedConcurrency[idx] <= 0 {
			continue
		}
		bucketStart, bucketEnd := bucketRange(opts.Domain, domainDuration, bucketCount, idx)
		bucketStart = math.Max(bucketStart, math.Max(clip.Start, opts.Domain.Start))
		bucketEnd = math.Min(bucketEnd, math.Min(clip.End, opts.Domain.End))
		if bucketEnd <= bucketStart {
			continue
		}
		ret = append(ret, Bucket{
			Start: bucketStart,
			End:   bucketEnd,
			Avg:   weightedConcurrency[idx] / (bucketEnd - bucketStart),
			Peak:  peakConcurrency[idx],
		})
	}
	return ret
}

func bucketRange(domain Range, domainDuration float64, bucketCount, idx int) (float64, float64) {
	bucketStart := domain.Start + domainDuration*float64(idx)/float64(bucketCount)
	bucketEnd := domain.Start + domainDuration*float64(idx+1)/float64(bucketCount)
	if idx == bucketCount-1 {
		bucketEnd = domain.End
	}
	return bucketStart, bucketEnd
}

func bucketIndexFloor(moment float64, domain Range, bucketCount int) int {
	if domain.End <= domain.Start || bucketCount <= 0 {
		return 0
	}
	relative := moment - domain.Start
	total := domain.End - domain.Start
	return int(math.Floor(relative * float64(bucketCount) / total))
}

func bucketIndexCeil(moment float64, domain Range, bucketCount int) int {
	if domain.End <= domain.Start || bucketCount <= 0 {
		return 0
	}
	relative := moment - domain.Start
	total := domain.End - domain.Start
	return int(math.Ceil(relative * float64(bucketCount) / total))
}
