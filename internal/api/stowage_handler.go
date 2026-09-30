package api

import (
	"errors"
	"net/http"

	"github.com/wyw14/cry-96/internal/model"
)

// writeStowageExceeded keeps the existing plan semantics intact: no plan was
// created, and the body carries the precise vessel and occupancy overflow.
func writeStowageExceeded(w http.ResponseWriter, exceedance *model.ExceedanceError) {
	writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
		"error":   exceedance.Error(),
		"code":    "stowage_exceeded",
		"stowage": exceedance.Report,
	})
}

func listStowageConfigHandler(runtime *Runtime) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		configs := runtime.Schedule.StowageConfigs(runtime.Chambers)
		result := make(map[string]model.StowageConfig, len(configs))
		for _, config := range configs {
			result[config.ChamberID] = config
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func putStowageConfigHandler(runtime *Runtime) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input model.StowageConfig
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := runtime.Schedule.SetStowageConfig(input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, input)
	}
}

type stowagePreviewInput struct {
	ChamberID string        `json:"chamber_id"`
	Vessels   []vesselInput `json:"vessels"`
}

func stowagePreviewHandler(runtime *Runtime) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input stowagePreviewInput
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if len(input.Vessels) == 0 {
			writeError(w, http.StatusBadRequest, errors.New("preview requires at least one vessel"))
			return
		}
		vessels := make([]model.Vessel, 0, len(input.Vessels))
		for _, item := range input.Vessels {
			vessel, err := model.NewVessel(item.Name, item.CallSign, item.LengthM)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			vessels = append(vessels, vessel)
		}
		report, err := runtime.Schedule.Preview(input.ChamberID, vessels)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		// A preview is informational: an overflow report is still a success.
		writeJSON(w, http.StatusOK, report)
	}
}
