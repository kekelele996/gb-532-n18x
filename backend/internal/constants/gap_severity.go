package constants

import "fmt"

type GapSeverity string
type GapState string

const (
	SeverityMinor    GapSeverity = "minor"
	SeverityMajor    GapSeverity = "major"
	SeverityCritical GapSeverity = "critical"

	GapDetected      GapState = "detected"
	GapReviewed      GapState = "reviewed"
	GapAccepted      GapState = "accepted"
	GapFalsePositive GapState = "false_positive"
	GapResurveyed    GapState = "resurveyed"
	GapClosed        GapState = "closed"
)

var gapTransitions = map[GapState]map[GapState]struct{}{
	GapDetected:      {GapReviewed: {}},
	GapReviewed:      {GapAccepted: {}, GapFalsePositive: {}},
	GapAccepted:      {GapResurveyed: {}},
	GapFalsePositive: {GapClosed: {}},
	GapResurveyed:    {GapClosed: {}},
	GapClosed:        {},
}

func (s GapState) Valid() bool {
	_, ok := gapTransitions[s]
	return ok
}

func (s GapState) CanTransition(target GapState) bool {
	allowed, ok := gapTransitions[s]
	if !ok {
		return false
	}
	_, ok = allowed[target]
	return ok
}

// Schedulable reports whether a gap snapshot may join a resurvey task sheet.
// Only snapshots awaiting review (detected) or already reviewed qualify.
func (s GapState) Schedulable() bool {
	return s == GapDetected || s == GapReviewed
}

// SeverityRank orders severities for resurvey scheduling; lower ranks schedule first.
func SeverityRank(value GapSeverity) int {
	switch value {
	case SeverityCritical:
		return 0
	case SeverityMajor:
		return 1
	default:
		return 2
	}
}

func ParseGapState(value string) (GapState, error) {
	state := GapState(value)
	if !state.Valid() {
		return "", fmt.Errorf("unknown gap state %q", value)
	}
	return state, nil
}

func SeverityForRatio(ratio float64) GapSeverity {
	switch {
	case ratio >= 0.12:
		return SeverityCritical
	case ratio >= 0.05:
		return SeverityMajor
	default:
		return SeverityMinor
	}
}
