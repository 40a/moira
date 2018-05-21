package controller

import (
	"fmt"

	"github.com/moira-alert/moira"
	"github.com/moira-alert/moira/api"
	"github.com/moira-alert/moira/api/dto"
	"github.com/moira-alert/moira/checker"
	"github.com/moira-alert/moira/database"
	"github.com/moira-alert/moira/target"
)

// GetTriggerEvaluationResult evaluates every target in trigger and returns
// result, separated on main and additional targets metrics
func GetTriggerEvaluationResult(dataBase moira.Database, from, to int64, triggerID string) (*checker.TriggerTimeSeries, *moira.Trigger, error) {
	trigger, err := dataBase.GetTrigger(triggerID)
	if err != nil {
		return nil, nil, err
	}
	triggerMetrics := &checker.TriggerTimeSeries{
		Main:       make([]*target.TimeSeries, 0),
		Additional: make([]*target.TimeSeries, 0),
	}
	isSimpleTrigger := trigger.IsSimple()
	for i, tar := range trigger.Targets {
		result, err := target.EvaluateTarget(dataBase, tar, from, to, isSimpleTrigger)
		if err != nil {
			return nil, &trigger, err
		}
		if i == 0 {
			triggerMetrics.Main = result.TimeSeries
		} else {
			triggerMetrics.Additional = append(triggerMetrics.Additional, result.TimeSeries...)
		}
	}
	return triggerMetrics, &trigger, nil
}

// GetTriggerMetrics gets all trigger metrics values, default values from: now - 10min, to: now
func GetTriggerMetrics(dataBase moira.Database, from, to int64, triggerID string) (*dto.TriggerMetrics, *api.ErrorResponse) {
	tts, _, err := GetTriggerEvaluationResult(dataBase, from, to, triggerID)
	if err != nil {
		if err == database.ErrNil {
			return nil, api.ErrorInvalidRequest(fmt.Errorf("trigger not found"))
		}
		return nil, api.ErrorInternalServer(err)
	}
	triggerMetrics := dto.TriggerMetrics{
		Main:       make(map[string][]*moira.MetricValue),
		Additional: make(map[string][]*moira.MetricValue),
	}
	for _, timeSeries := range tts.Main {
		values := make([]*moira.MetricValue, 0)
		for i := 0; i < len(timeSeries.Values); i++ {
			timestamp := int64(timeSeries.StartTime + int32(i)*timeSeries.StepTime)
			value := timeSeries.GetTimestampValue(timestamp)
			if !checker.IsInvalidValue(value) {
				values = append(values, &moira.MetricValue{Value: value, Timestamp: timestamp})
			}
		}
		triggerMetrics.Main[timeSeries.Name] = values
	}
	for _, timeSeries := range tts.Additional {
		values := make([]*moira.MetricValue, 0)
		for i := 0; i < len(timeSeries.Values); i++ {
			timestamp := int64(timeSeries.StartTime + int32(i)*timeSeries.StepTime)
			value := timeSeries.GetTimestampValue(timestamp)
			if !checker.IsInvalidValue(value) {
				values = append(values, &moira.MetricValue{Value: value, Timestamp: timestamp})
			}
		}
		triggerMetrics.Additional[timeSeries.Name] = values
	}
	return &triggerMetrics, nil
}

// DeleteTriggerMetric deletes metric from last check and all trigger patterns metrics
func DeleteTriggerMetric(dataBase moira.Database, metricName string, triggerID string) *api.ErrorResponse {
	trigger, err := dataBase.GetTrigger(triggerID)
	if err != nil {
		if err == database.ErrNil {
			return api.ErrorInvalidRequest(fmt.Errorf("trigger not found"))
		}
		return api.ErrorInternalServer(err)
	}

	if err = dataBase.AcquireTriggerCheckLock(triggerID, 10); err != nil {
		return api.ErrorInternalServer(err)
	}
	defer dataBase.DeleteTriggerCheckLock(triggerID)

	lastCheck, err := dataBase.GetTriggerLastCheck(triggerID)
	if err != nil {
		if err == database.ErrNil {
			return api.ErrorInvalidRequest(fmt.Errorf("trigger check not found"))
		}
		return api.ErrorInternalServer(err)
	}
	_, ok := lastCheck.Metrics[metricName]
	if ok {
		delete(lastCheck.Metrics, metricName)
		lastCheck.UpdateScore()
	}
	if err = dataBase.RemovePatternsMetrics(trigger.Patterns); err != nil {
		return api.ErrorInternalServer(err)
	}
	if err = dataBase.SetTriggerLastCheck(triggerID, &lastCheck); err != nil {
		return api.ErrorInternalServer(err)
	}
	return nil
}
