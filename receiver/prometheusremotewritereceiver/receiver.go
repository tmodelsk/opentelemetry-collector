// Copyright Sabre

package prometheusremotewritereceiver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage/remote"
	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/obsreport"
)

const (
	receiverTransport = "http"
	receiverFormat    = "protobuf"
)

// Receiver - remote write
type Receiver struct {
	host         component.Host
	nextConsumer consumer.Metrics

	mu         sync.Mutex
	startOnce  sync.Once
	stopOnce   sync.Once
	shutdownWG sync.WaitGroup

	server *http.Server
	config *Config
	logger *zap.Logger
}

// New - remote write
func New(config *Config, consumer consumer.Metrics, logger *zap.Logger) (*Receiver, error) {
	zr := &Receiver{
		nextConsumer: consumer,
		config:       config,
		logger:       logger,
	}
	return zr, nil
}

// Start - remote write
func (rec *Receiver) Start(_ context.Context, host component.Host) error {
	if host == nil {
		return errors.New("nil host")
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	var err = componenterror.ErrAlreadyStarted
	rec.startOnce.Do(func() {
		err = nil
		rec.host = host
		rec.server = rec.config.HTTPServerSettings.ToServer(rec)
		var listener net.Listener
		listener, err = rec.config.HTTPServerSettings.ToListener()
		if err != nil {
			return
		}
		rec.shutdownWG.Add(1)
		go func() {
			defer rec.shutdownWG.Done()
			if errHTTP := rec.server.Serve(listener); errHTTP != http.ErrServerClosed {
				host.ReportFatalError(errHTTP)
			}
		}()
	})
	return err
}

func (rec *Receiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req, err := remote.DecodeWriteRequest(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := obsreport.ReceiverContext(r.Context(), typeStr, receiverTransport)
	ctx = obsreport.StartMetricsReceiveOp(ctx, typeStr, receiverTransport)

	reg := regexp.MustCompile(`(\w+)_(\w+)_(\w+)\z`)
	pms := pdata.NewMetrics()
	for _, ts := range req.Timeseries {
		prm := pdata.NewResourceMetrics()
		pilm := pdata.NewInstrumentationLibraryMetrics()
		prm.InstrumentationLibraryMetrics().Append(pilm)
		pms.ResourceMetrics().Append(prm)

		for _, l := range ts.Labels {
			if l.Name == "XXX" {
				prm.Resource().Attributes().Insert(l.Name, pdata.NewAttributeValueString(l.Value))
			}
		}

		metricName := finaName(ts.Labels)
		pm := pdata.NewMetric()
		pm.SetName(metricName)
		rec.logger.Debug("Metric name", zap.String("metric_name", pm.Name()))

		match := reg.FindStringSubmatch(metricName)
		metricsType := ""
		if len(match) > 1 {
			lastSuffixInMetricName := match[len(match)-1]
			if IsValidSuffix(lastSuffixInMetricName) {
				metricsType = lastSuffixInMetricName
				if len(match) > 2 {
					secondSuffixInMetricName := match[len(match)-2]
					if IsValidUnit(secondSuffixInMetricName) {
						pm.SetUnit(secondSuffixInMetricName)
					}
				}
			} else if IsValidUnit(lastSuffixInMetricName) {
				pm.SetUnit(lastSuffixInMetricName)
			}
		}
		rec.logger.Debug("Metric unit", zap.String("metric name", pm.Name()), zap.String("metric_unit", pm.Unit()))

		for _, s := range ts.Samples {
			ppoint := pdata.NewDoubleDataPoint()
			ppoint.SetValue(s.Value)
			ppoint.SetTimestamp(pdata.TimestampFromTime(time.Unix(0, s.Timestamp*int64(time.Millisecond))))
			for _, l := range ts.Labels {
				ppoint.LabelsMap().Insert(l.Name, l.Value)
			}
			if metricsType == "sum" {
				pm.SetDataType(pdata.MetricDataTypeDoubleSum)
				pm.DoubleSum().DataPoints().Append(ppoint)
			} else {
				pm.SetDataType(pdata.MetricDataTypeDoubleGauge)
				pm.DoubleGauge().DataPoints().Append(ppoint)
			}
			rec.logger.Debug("Metric sample",
				zap.String("metric_name", pm.Name()),
				zap.String("metric_unit", pm.Unit()),
				zap.Float64("metric_value", ppoint.Value()),
				zap.Time("metric_timestamp", ppoint.Timestamp().AsTime()),
				zap.String("metric_labels", fmt.Sprintf("%#v", ppoint.LabelsMap())),
			)
		}
		pilm.Metrics().Append(pm)
	}

	metricCount, dataPointCount := pms.MetricAndDataPointCount()
	if metricCount != 0 {
		err = rec.nextConsumer.ConsumeMetrics(ctx, pms)
	}
	obsreport.EndMetricsReceiveOp(ctx, receiverFormat, dataPointCount, err)
	w.WriteHeader(http.StatusAccepted)
}

// Shutdown - remote write
func (rec *Receiver) Shutdown(context.Context) error {
	var err = componenterror.ErrAlreadyStopped
	rec.stopOnce.Do(func() {
		err = rec.server.Close()
		rec.shutdownWG.Wait()
	})
	return err
}

func finaName(labels []prompb.Label) (ret string) {
	for _, label := range labels {
		if label.Name == "__name__" {
			return label.Value
		}
	}
	return "error"
}

// IsValidSuffix - remote write
func IsValidSuffix(suffix string) bool {
	switch suffix {
	case
		"max",
		"sum",
		"count",
		"total":
		return true
	}
	return false
}

// IsValidUnit - remote write
func IsValidUnit(unit string) bool {
	switch unit {
	case
		"seconds",
		"bytes":
		return true
	}
	return false
}
