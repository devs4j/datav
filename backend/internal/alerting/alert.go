package alerting

import (
	"encoding/json"
	"time"

	"github.com/code-creatively/datav/backend/pkg/db"
	"github.com/code-creatively/datav/backend/pkg/models"
	"github.com/code-creatively/datav/backend/pkg/utils/simplejson"
)

func UpdateDashboardAlerts(dash *models.Dashboard) error {
	extractor := &DashAlertExtractor{dash}
	alerts, err := extractor.GetAlerts()
	if err != nil {
		logger.Warn("extrac alerts error", "error", err)
		return err
	}

	// delete old alerts
	_, err = db.SQL.Exec("DELETE FROM alert WHERE dashboard_id=?", dash.Id)
	if err != nil {
		logger.Warn("delete dashboard alert error", "error", err)
		return err
	}

	// delete old alerts notification state
	_, err = db.SQL.Exec("DELETE FROM alert_notification_state WHERE dashboard_id=?", dash.Id)
	if err != nil {
		logger.Warn("delete alert notification state error", "error", err)
		return err
	}

	now := time.Now()
	for _, alert := range alerts {
		alert.Created = now
		alert.Updated = now
		alert.State = models.AlertStateUnknown
		alert.NewStateDate = now

		settings, _ := alert.Settings.Encode()
		_, err = db.SQL.Exec("INSERT INTO alert (dashboard_id,panel_id,name,message,state,new_state_date,state_changes,frequency,for,handler,silenced,execution_error,eval_data,settings,created,updated) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
			alert.DashboardId, alert.PanelId, alert.Name, alert.Message, alert.State, alert.NewStateDate,
			alert.StateChanges, alert.Frequency, alert.For, alert.Handler, alert.Silenced, alert.ExecutionError, alert.EvalData, settings, alert.Created, alert.Updated)
		if err != nil {
			logger.Warn("insert dashboard alert error", "error", err)
			return err
		}
	}

	return nil
}

func GetAllAlerts() ([]*models.Alert, error) {
	rows, err := db.SQL.Query("SELECT * FROM alert")
	if err != nil {
		return nil, err
	}

	alerts := make([]*models.Alert, 0)
	for rows.Next() {
		alert := &models.Alert{}
		var settings []byte
		var evalData []byte
		err := rows.Scan(&alert.Id, &alert.DashboardId, &alert.PanelId, &alert.Name, &alert.Message,
			&alert.State, &alert.NewStateDate, &alert.StateChanges, &alert.Frequency, &alert.For,
			&alert.Handler, &alert.Silenced, &alert.ExecutionError, &evalData, &alert.EvalDate, &settings,
			&alert.Created, &alert.Updated)
		if err != nil {
			logger.Warn("scan all alerts error", "error", err)
		}

		err = json.Unmarshal(settings, &alert.Settings)
		if err != nil {
			logger.Warn("unmarshal all alerts error", "error", err)
		}

		if evalData != nil {
			err = json.Unmarshal(evalData, &alert.EvalData)
			if err != nil {
				logger.Warn("unmarshal all alerts error", "error", err)
			}
		}

		alerts = append(alerts, alert)
	}

	return alerts, nil
}

func GetAlert(id int64) (*models.Alert, error) {
	alert := &models.Alert{}
	var settings []byte
	var evalData []byte
	err := db.SQL.QueryRow("SELECT * FROM alert WHERE id=?", id).Scan(&alert.Id, &alert.DashboardId, &alert.PanelId, &alert.Name, &alert.Message,
		&alert.State, &alert.NewStateDate, &alert.StateChanges, &alert.Frequency, &alert.For,
		&alert.Handler, &alert.Silenced, &alert.ExecutionError, &evalData, &alert.EvalDate, &settings,
		&alert.Created, &alert.Updated)
	if err != nil {
		logger.Warn("get alert error", "error", err)
		return nil, err
	}

	err = json.Unmarshal(settings, &alert.Settings)
	if err != nil {
		logger.Warn("unmarshal all alerts error", "error", err)
		return nil, err
	}

	if evalData != nil {
		err = json.Unmarshal(evalData, &alert.EvalData)
		if err != nil {
			logger.Warn("unmarshal all alerts error", "error", err)
			return nil, err
		}
	}

	return alert, nil
}

func UpdateAlert(alert *models.Alert) error {
	now := time.Now()
	evalData, _ := alert.EvalData.Encode()
	_, err := db.SQL.Exec("UPDATE alert SET state=?, new_state_date=?, state_changes=?, eval_data=?, execution_error=?, updated=? WHERE id=?",
		alert.State, alert.NewStateDate, alert.StateChanges, evalData, alert.ExecutionError, now, alert.Id)
	if err != nil {
		logger.Warn("update alert error", "error", err)
		return err
	}

	return nil
}

func SetAlertState(alertId int64, state models.AlertStateType, annotationData *simplejson.Json, executionError string) (*models.Alert, error) {
	alert, err := GetAlert(alertId)
	if err != nil {
		return nil, err
	}

	if alert.State == models.AlertStatePaused {
		return nil, models.ErrCannotChangeStateOnPausedAlert
	}

	if alert.State == state {
		return nil, models.ErrRequiresNewState
	}

	alert.State = state
	alert.StateChanges++
	alert.NewStateDate = time.Now()
	alert.EvalData = annotationData

	if executionError == "" {
		alert.ExecutionError = "" //without this space, xorm skips updating this field
	} else {
		alert.ExecutionError = executionError
	}

	err = UpdateAlert(alert)
	if err != nil {
		return nil, err
	}

	return alert, nil
}