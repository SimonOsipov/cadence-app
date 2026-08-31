package measurements

import (
	"slices"
	"testing"
)

// The seam these exist for: a string becomes a member of a closed set here and nowhere else.
// `type Metric string` makes a foreign value representable, and `body_fat` or `hips` reaching
// the wire costs two metrics of eight silently — KMP's `Metric.fromCode` answers null and the
// overview stays well-formed.
func TestAValueOffTheSetIsRefusedRatherThanRepresented(t *testing.T) {
	for _, refused := range []string{"WEIGHT", "Weight", "", " weight", "body_fat", "hips", "thigh"} {
		if got, ok := ParseMetric(refused); ok {
			t.Errorf("ParseMetric(%q) gave %q", refused, got)
		}
	}
	for _, refused := range []string{"MANUAL", "Manual", "", "healthConnect", "health-connect", "google_fit"} {
		if got, ok := ParseSource(refused); ok {
			t.Errorf("ParseSource(%q) gave %q", refused, got)
		}
	}
}

// The accept side against the declared sets rather than against a repeated literal: a
// constant renamed and a parser left behind is exactly what one list would hide.
func TestEveryDeclaredValueParsesBackToItself(t *testing.T) {
	for _, metric := range Metrics() {
		if got, ok := ParseMetric(string(metric)); !ok || got != metric {
			t.Errorf("ParseMetric(%q) gave %q, %v", metric, got, ok)
		}
	}
	for _, source := range Sources() {
		if got, ok := ParseSource(string(source)); !ok || got != source {
			t.Errorf("ParseSource(%q) gave %q, %v", source, got, ok)
		}
	}
}

// Written out rather than read off the constants: an expectation derived from the thing under
// test moves with it and pins nothing. The order is pinned with them — the overview
// answers in it, and it is the KMP enum's.
func TestTheCodesOnTheWireAreTheWrittenOutOnes(t *testing.T) {
	if got := Metrics(); !slices.Equal(got, []Metric{
		"weight", "hrv", "rhr", "sleep", "bodyfat", "waist", "hip", "chest",
	}) {
		t.Errorf("the metrics are %v", got)
	}
	if got := Sources(); !slices.Equal(got, []Source{"manual", "healthkit", "health_connect"}) {
		t.Errorf("the sources are %v", got)
	}
	if got := WritableMetrics(); !slices.Equal(got, []Metric{
		"weight", "hrv", "rhr", "bodyfat", "waist", "hip", "chest",
	}) {
		t.Errorf("the writable metrics are %v", got)
	}
}

// The two metric sets against each other, and the exclusion named rather than counted: a
// seven that dropped the chest instead of the sleep is exactly as long as the right one, and
// a writable metric the parser refuses is a value nobody can send.
func TestTheOneMetricNoPatientTypesIsTheSleepScore(t *testing.T) {
	var withheld []Metric
	for _, metric := range Metrics() {
		if !slices.Contains(WritableMetrics(), metric) {
			withheld = append(withheld, metric)
		}
	}
	if !slices.Equal(withheld, []Metric{MetricSleep}) {
		t.Errorf("the metrics no patient can type are %v", withheld)
	}

	for _, metric := range WritableMetrics() {
		if _, ok := ParseMetric(string(metric)); !ok {
			t.Errorf("%q can be written and cannot be parsed", metric)
		}
		if !writable(metric) {
			t.Errorf("%q is in the set and the guard refuses it", metric)
		}
	}
	if writable(MetricSleep) {
		t.Error("the guard admits the sleep score")
	}
}
