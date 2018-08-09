package heartbeat

import (
	"time"

	"github.com/patrickmn/go-cache"
	"gopkg.in/tomb.v2"

	"github.com/moira-alert/moira"
	"github.com/moira-alert/moira/metrics/graphite"
)

const (
	metricsMatchedCacheKey       = "metricsMatched"
	cacheCleanupInterval         = time.Minute * 5
	cacheValueExpirationDuration = time.Minute
)

// Worker is heartbeat worker realization
type Worker struct {
	database moira.Database
	metrics  *graphite.FilterMetrics
	logger   moira.Logger
	tomb     tomb.Tomb
	cache    *cache.Cache
}

// NewHeartbeatWorker creates new worker
func NewHeartbeatWorker(database moira.Database, metrics *graphite.FilterMetrics, logger moira.Logger) *Worker {
	return &Worker{
		database: database,
		metrics:  metrics,
		logger:   logger,
		cache:    cache.New(cacheValueExpirationDuration, cacheCleanupInterval),
	}
}

// Start every 5 second takes TotalMetricsReceived metrics and save it to database, for self-checking
func (worker *Worker) Start() {
	receivedCount := worker.metrics.TotalMetricsReceived.Count()
	worker.tomb.Go(func() error {
		metricsReceivedCheckTicker := time.NewTicker(time.Second * 5)
		metricsMatchedCheckTicker := time.NewTicker(time.Minute)
		for {
			select {
			case <-worker.tomb.Dying():
				worker.logger.Info("Moira Filter Heartbeat stopped")
				return nil
			case <-metricsReceivedCheckTicker.C:
				newReceivedCount := worker.metrics.TotalMetricsReceived.Count()
				worker.logger.Debugf("Update heartbeat count, old value: %v, new value: %v", receivedCount, newReceivedCount)
				if newReceivedCount != receivedCount {
					if err := worker.database.UpdateMetricsHeartbeat(); err != nil {
						worker.logger.Infof("Save state failed: %s", err.Error())
					} else {
						receivedCount = newReceivedCount
					}
				}
			case <-metricsMatchedCheckTicker.C:
				dataBaseMatchedCount, err := worker.database.GetMatchedMetricsUpdatesCount()
				if err != nil {
					worker.logger.Error("Can't perform check on matched metrics counter. Setting Moira Notifier state to ERROR")
					worker.database.SetNotifierState("ERROR")
				} else {
					if dataBaseMatchedCount != 0 {
						newMatchedCountPerInterval := worker.metrics.MatchingMetricsReceived.Count() - dataBaseMatchedCount
						if previouslyMatchedPerInterval, found := worker.cache.Get(metricsMatchedCacheKey); found {
							previouslyMatchedPerIntervalVal := previouslyMatchedPerInterval.(int64)
							if newMatchedCountPerInterval < 0.5*previouslyMatchedPerIntervalVal {
								worker.logger.Errorf("Found 50% less matched metrics than minute ago. Previously: %d. Now: %d", previouslyMatchedPerIntervalVal, newMatchedCountPerInterval)
							}
						}
						worker.cache.Set(metricsMatchedCacheKey, newMatchedCountPerInterval, cacheValueExpirationDuration)
					}
				}
			}
		}
	})
	worker.logger.Info("Moira Filter Heartbeat started")
}

// Stop heartbeat worker
func (worker *Worker) Stop() error {
	worker.tomb.Kill(nil)
	return worker.tomb.Wait()
}
