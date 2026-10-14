package provider

import (
	_ "embed"
	"encoding/json"
	"sync"
)

// A name/code registry, not an administrative-boundary or flood dataset.
// Source and retrieval fingerprint are stored with the embedded snapshot.
//
//go:embed data/thai_district_codes.json
var thaiDistrictCodeSnapshot []byte
var districtCodeOnce sync.Once
var districtCodeNames map[string]int

func registryDistrictCode(location ReverseGeocodingResult) int {
	districtCodeOnce.Do(func() {
		districtCodeNames = map[string]int{}
		var snapshot struct {
			Records []struct {
				Province string `json:"province"`
				District string `json:"district"`
				Code     int    `json:"code"`
			} `json:"records"`
		}
		if json.Unmarshal(thaiDistrictCodeSnapshot, &snapshot) != nil {
			return
		}
		for _, r := range snapshot.Records {
			if r.Code < 1000 || r.Code > 9999 || r.Province == "" || r.District == "" {
				continue
			}
			key := annualAreaName(r.Province) + "|" + annualAreaName(r.District)
			if code, exists := districtCodeNames[key]; exists && code != r.Code {
				districtCodeNames[key] = 0
			} else if !exists {
				districtCodeNames[key] = r.Code
			}
		}
	})
	return districtCodeNames[annualAreaName(location.Province)+"|"+annualAreaName(location.District)]
}
