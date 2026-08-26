package httpapi

import (
	"net/http"

	"tape-preservation-incident-api/internal/app"
)

const maxRequestBody = 1 << 20

type API struct {
	service *app.Service
	mux     *http.ServeMux
}

func New(service *app.Service) *API {
	api := &API{service: service, mux: http.NewServeMux()}
	api.routes()
	return api
}

func (a *API) Handler() http.Handler {
	return a.correlation(a.securityHeaders(a.mux))
}

func (a *API) routes() {
	a.mux.HandleFunc("GET /healthz", a.Health)
	a.mux.HandleFunc("POST /api/v1/incidents", a.CreateIncident)
	a.mux.HandleFunc("GET /api/v1/incidents/{incident_id}", a.GetIncident)
	a.mux.HandleFunc("POST /api/v1/incidents/{incident_id}/assessments", a.AddAssessment)
	a.mux.HandleFunc("POST /api/v1/incidents/{incident_id}/assessments/batch", a.AddAssessmentBatch)
	a.mux.HandleFunc("POST /api/v1/incidents/{incident_id}/boundary", a.FreezeBoundary)
	a.mux.HandleFunc("POST /api/v1/incidents/{incident_id}/boundary/preflight", a.BoundaryPreflight)
	a.mux.HandleFunc("POST /api/v1/incidents/{incident_id}/plans", a.SubmitPlan)
	a.mux.HandleFunc("POST /api/v1/incidents/{incident_id}/plans/approval", a.ApprovePlan)
	a.mux.HandleFunc("POST /api/v1/incidents/{incident_id}/treatments", a.AddTreatment)
	a.mux.HandleFunc("POST /api/v1/incidents/{incident_id}/treatments/{record_id}/interruptions", a.InterruptTreatment)
	a.mux.HandleFunc("POST /api/v1/incidents/{incident_id}/treatments/{record_id}/resume", a.ResumeTreatment)
	a.mux.HandleFunc("POST /api/v1/incidents/{incident_id}/treatments/{record_id}/complete", a.CompleteTreatment)
	a.mux.HandleFunc("POST /api/v1/incidents/{incident_id}/verifications", a.AddVerification)
	a.mux.HandleFunc("POST /api/v1/incidents/{incident_id}/decisions", a.Decide)
	a.mux.HandleFunc("POST /api/v1/incidents/{incident_id}/seal", a.Seal)
	a.mux.HandleFunc("GET /api/v1/incidents/{incident_id}/timeline", a.GetTimeline)
	a.mux.HandleFunc("GET /api/v1/incidents/{incident_id}/archive/verify", a.VerifyArchive)
	a.mux.HandleFunc("GET /api/v1/incidents/{incident_id}/archive/manifest", a.GetArchiveManifest)
	a.mux.HandleFunc("GET /api/v1/incidents/{incident_id}/rounds/{round_number}", a.GetRound)
}
