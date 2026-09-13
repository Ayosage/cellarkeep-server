package domain

import "fmt"

type Metric string

type MetricInfo struct {
	Key       Metric
	Label     string
	Unit      string
	Precision int
	Min, Max  float64
}

var registry = []MetricInfo{
	{"gravity", "Specific gravity", "SG", 3, 0.9, 1.2},
	{"brix", "Brix", "°Bx", 1, 0, 50},
	{"density", "Density", "g/cm³", 4, 0.9, 1.2},
	{"ph", "pH", "pH", 2, 2, 5},
	{"ta", "Titratable acidity", "g/L tartaric", 2, 0, 20},
	{"free_so2", "Free SO₂", "mg/L", 0, 0, 200},
	{"total_so2", "Total SO₂", "mg/L", 0, 0, 400},
	{"malic_acid", "Malic acid", "g/L", 2, 0, 10},
	{"abv", "Alcohol by volume", "% v/v", 1, 0, 25},
	{"temp", "Temperature", "°C", 1, -10, 60},
	{"humidity", "Relative humidity", "%RH", 0, 0, 100},
}

func Metrics() []MetricInfo { return append([]MetricInfo(nil), registry...) }

func LookupMetric(key string) (MetricInfo, bool) {
	for _, m := range registry {
		if string(m.Key) == key {
			return m, true
		}
	}
	return MetricInfo{}, false
}

func (m MetricInfo) Validate(v float64) error {
	if v < m.Min || v > m.Max {
		return invalid("value", fmt.Sprintf("%s must be between %v and %v %s", m.Key, m.Min, m.Max, m.Unit))
	}
	return nil
}
