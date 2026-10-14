package provider

import "testing"

func TestDistrictRegistryMatchesProvinceAndDistrictWithoutSubstitution(t *testing.T) {
	for _, test := range []struct {
		province, district string
		code               int
	}{{"กรุงเทพมหานคร", "เขตห้วยขวาง", 1017}, {"จังหวัดพระนครศรีอยุธยา", "อำเภอพระนครศรีอยุธยา", 1401}, {"บึงกาฬ", "เซกา", 3804}, {"เชียงใหม่", "ห้วยขวาง", 0}, {"จังหวัดที่ไม่รู้จัก", "ห้วยขวาง", 0}} {
		if got := registryDistrictCode(ReverseGeocodingResult{Province: test.province, District: test.district}); got != test.code {
			t.Fatalf("registry returned %d for %s/%s", got, test.province, test.district)
		}
	}
}
