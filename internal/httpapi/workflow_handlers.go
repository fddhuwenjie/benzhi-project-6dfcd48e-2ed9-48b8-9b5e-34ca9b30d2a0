package httpapi

import (
	"net/http"

	"tape-preservation-incident-api/internal/app"
)

func (a *API) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) CreateIncident(w http.ResponseWriter, r *http.Request) {
	var cmd app.CreateIncidentCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := a.service.CreateIncident(r.Context(), cmd)
	if err != nil {
		writeError(w, r, err)
		return
	}
	status := http.StatusCreated
	if result.Replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, result)
}

func (a *API) AddAssessment(w http.ResponseWriter, r *http.Request) {
	var cmd app.AddAssessmentCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := a.service.AddAssessment(r.Context(), r.PathValue("incident_id"), cmd)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) AddAssessmentBatch(w http.ResponseWriter, r *http.Request) {
	var cmd app.AddAssessmentBatchCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := a.service.AddAssessmentBatch(r.Context(), r.PathValue("incident_id"), cmd)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) BoundaryPreflight(w http.ResponseWriter, r *http.Request) {
	var cmd app.BoundaryPreflightCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := a.service.BoundaryPreflight(r.Context(), r.PathValue("incident_id"), cmd)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) FreezeBoundary(w http.ResponseWriter, r *http.Request) {
	var cmd app.FreezeBoundaryCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := a.service.FreezeBoundary(r.Context(), r.PathValue("incident_id"), cmd)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) SubmitPlan(w http.ResponseWriter, r *http.Request) {
	var cmd app.SubmitPlanCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := a.service.SubmitPlan(r.Context(), r.PathValue("incident_id"), cmd)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) ApprovePlan(w http.ResponseWriter, r *http.Request) {
	var cmd app.ApprovePlanCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := a.service.ApprovePlan(r.Context(), r.PathValue("incident_id"), cmd)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) AddTreatment(w http.ResponseWriter, r *http.Request) {
	var cmd app.AddTreatmentCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := a.service.AddTreatment(r.Context(), r.PathValue("incident_id"), cmd)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) InterruptTreatment(w http.ResponseWriter, r *http.Request) {
	var cmd app.InterruptTreatmentCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := a.service.InterruptTreatment(r.Context(), r.PathValue("incident_id"), r.PathValue("record_id"), cmd)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) ResumeTreatment(w http.ResponseWriter, r *http.Request) {
	var cmd app.ResumeTreatmentCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := a.service.ResumeTreatment(r.Context(), r.PathValue("incident_id"), r.PathValue("record_id"), cmd)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) CompleteTreatment(w http.ResponseWriter, r *http.Request) {
	var cmd app.CompleteTreatmentCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := a.service.CompleteTreatment(r.Context(), r.PathValue("incident_id"), r.PathValue("record_id"), cmd)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) AddVerification(w http.ResponseWriter, r *http.Request) {
	var cmd app.AddVerificationCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := a.service.AddVerification(r.Context(), r.PathValue("incident_id"), cmd)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) Decide(w http.ResponseWriter, r *http.Request) {
	var cmd app.DecideCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := a.service.Decide(r.Context(), r.PathValue("incident_id"), cmd)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) Seal(w http.ResponseWriter, r *http.Request) {
	var cmd app.SealCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := a.service.Seal(r.Context(), r.PathValue("incident_id"), cmd)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
