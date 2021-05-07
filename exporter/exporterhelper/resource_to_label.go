// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exporterhelper

import (
	"go.opentelemetry.io/collector/consumer/pdata"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
)

// ResourceToTelemetrySettings defines configuration for converting resource attributes to metric labels.
type ResourceToTelemetrySettings struct {
	// Enabled indicates whether to not convert resource attributes to metric labels
	Enabled bool `mapstructure:"enabled"`
	// Set of attribute names to exclude when converting resource attributes to metric labels
	ExcludeAttributes []string `mapstructure:"exclude_attributes"`
}

// defaultResourceToTelemetrySettings returns the default settings for ResourceToTelemetrySettings.
func defaultResourceToTelemetrySettings() ResourceToTelemetrySettings {
	return ResourceToTelemetrySettings{
		Enabled:           false,
		ExcludeAttributes: []string{},
	}
}

// convertResourceToLabels converts all resource attributes to metric labels except excludeAttributes set
func convertResourceToLabels(md pdata.Metrics, excludeAttributes map[string]bool) pdata.Metrics {
	cloneMd := md.Clone()
	rms := cloneMd.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		resource := rms.At(i).Resource()

		labelMap := extractLabelsFromResource(&resource, excludeAttributes)

		ilms := rms.At(i).InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)
			metricSlice := ilm.Metrics()
			for k := 0; k < metricSlice.Len(); k++ {
				metric := metricSlice.At(k)
				addLabelsToMetric(&metric, labelMap)
			}
		}
	}
	return cloneMd
}

// extractAttributesFromResource extracts the attributes from a given resource and
// returns them as a StringMap.
func extractLabelsFromResource(resource *pdata.Resource, excludeAttributes map[string]bool) pdata.StringMap {
	labelMap := pdata.NewStringMap()

	attrMap := resource.Attributes()
	attrMap.ForEach(func(k string, av pdata.AttributeValue) {
		_, shouldBeSkipped := excludeAttributes[k]
		if !shouldBeSkipped {
			stringLabel := tracetranslator.AttributeValueToString(av, false)
			labelMap.Upsert(k, stringLabel)
		}
	})
	return labelMap
}

// addLabelsToMetric adds additional labels to the given metric
func addLabelsToMetric(metric *pdata.Metric, labelMap pdata.StringMap) {
	switch metric.DataType() {
	case pdata.MetricDataTypeIntGauge:
		addLabelsToIntDataPoints(metric.IntGauge().DataPoints(), labelMap)
	case pdata.MetricDataTypeDoubleGauge:
		addLabelsToDoubleDataPoints(metric.DoubleGauge().DataPoints(), labelMap)
	case pdata.MetricDataTypeIntSum:
		addLabelsToIntDataPoints(metric.IntSum().DataPoints(), labelMap)
	case pdata.MetricDataTypeDoubleSum:
		addLabelsToDoubleDataPoints(metric.DoubleSum().DataPoints(), labelMap)
	case pdata.MetricDataTypeIntHistogram:
		addLabelsToIntHistogramDataPoints(metric.IntHistogram().DataPoints(), labelMap)
	case pdata.MetricDataTypeHistogram:
		addLabelsToDoubleHistogramDataPoints(metric.Histogram().DataPoints(), labelMap)
	}
}

func addLabelsToIntDataPoints(ps pdata.IntDataPointSlice, newLabelMap pdata.StringMap) {
	for i := 0; i < ps.Len(); i++ {
		joinStringMaps(newLabelMap, ps.At(i).LabelsMap())
	}
}

func addLabelsToDoubleDataPoints(ps pdata.DoubleDataPointSlice, newLabelMap pdata.StringMap) {
	for i := 0; i < ps.Len(); i++ {
		joinStringMaps(newLabelMap, ps.At(i).LabelsMap())
	}
}

func addLabelsToIntHistogramDataPoints(ps pdata.IntHistogramDataPointSlice, newLabelMap pdata.StringMap) {
	for i := 0; i < ps.Len(); i++ {
		joinStringMaps(newLabelMap, ps.At(i).LabelsMap())
	}
}

func addLabelsToDoubleHistogramDataPoints(ps pdata.HistogramDataPointSlice, newLabelMap pdata.StringMap) {
	for i := 0; i < ps.Len(); i++ {
		joinStringMaps(newLabelMap, ps.At(i).LabelsMap())
	}
}

func joinStringMaps(from, to pdata.StringMap) {
	from.ForEach(func(k, v string) {
		to.Upsert(k, v)
	})
}
