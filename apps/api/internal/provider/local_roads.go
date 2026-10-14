package provider

import (
	"encoding/json"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/roads"
)

func localRoadObservation(result roads.Result, radius int) Observation {
	raw, _ := json.Marshal(struct {
		osmRoadMetricValue
		DatasetID    string `json:"roadDatasetId"`
		Oneway       string `json:"nearestRoadOneway,omitempty"`
		Access       string `json:"nearestRoadAccess,omitempty"`
		MotorVehicle string `json:"nearestRoadMotorVehicle,omitempty"`
	}{osmRoadMetricValue{RadiusMeters: radius, MappedMajorRoadCount: result.Count, NearestMajorRoadMeters: result.Distance, NearestRoadName: result.Name, NearestRoadRef: result.Ref, NearestRoadClass: result.Class, RoadClassCounts: result.Counts}, result.DatasetID.String(), result.Oneway, result.Access, result.MotorVehicle})
	return Observation{MetricType: "road_accessibility", RawValue: raw, Status: domain.DataPreliminary,
		Source:      domain.DataSource{Name: "OpenStreetMap Thailand — local road snapshot", Type: "local_open_data_snapshot", ReferenceURI: "https://download.geofabrik.de/asia/thailand.html", RetrievedAt: result.ImportedAt, DatasetVersion: result.SourceTime.UTC().Format("2006-01-02T15:04:05Z"), License: "Open Data Commons Open Database License (ODbL) 1.0", Methodology: "Query an indexed local Thailand OSM extract within the requested radius. Nearest major-road distance is measured to the full road line using PostGIS geography in metres."},
		Assumptions: []string{"Mapped major-road proximity is a preliminary access proxy. It does not establish a legal driveway, turning permission or physical entry to this plot.", "OpenStreetMap coverage and access tags can be incomplete. Missing restriction tags do not prove unrestricted access.", "The source snapshot date is disclosed; snapshots older than 30 days are not used as the local primary source.", "Road-based traffic potential is an estimate, never an official vehicle count or AADT measurement."}}
}
