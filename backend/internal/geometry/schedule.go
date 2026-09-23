package geometry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// StableScheduleHash freezes the resurvey task sheet input so repeated
// generation over the same snapshot set yields the identical sheet.
func StableScheduleHash(algorithmVersion string, gapIDs []uint) string {
	values := append([]uint(nil), gapIDs...)
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	payload, _ := json.Marshal(struct {
		Algorithm string `json:"algorithm"`
		GapIDs    []uint `json:"gap_ids"`
	}{algorithmVersion, values})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
