package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"tape-preservation-incident-api/internal/preservation"
)

func (a *API) GetIncident(w http.ResponseWriter, r *http.Request) {
	incident, err := a.service.GetIncident(r.Context(), r.PathValue("incident_id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"incident": incident, "current_round": incident.CurrentRound, "rounds": incident.RoundSummaries()})
}

func (a *API) GetRound(w http.ResponseWriter, r *http.Request) {
	number, err := strconv.Atoi(r.PathValue("round_number"))
	if err != nil || number < 1 {
		writeError(w, r, preservation.Invalid("round_number", "必须为正整数"))
		return
	}
	round, err := a.service.GetRound(r.Context(), r.PathValue("incident_id"), number)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"round": round})
}

func (a *API) GetArchiveManifest(w http.ResponseWriter, r *http.Request) {
	expected := r.URL.Query().Get("expected_digest")
	if expected == "" {
		expected = r.Header.Get("X-Expected-Archive-Digest")
	}
	manifest, err := a.service.ArchiveManifest(r.Context(), r.PathValue("incident_id"), strings.Trim(expected, "\""))
	if err != nil {
		writeError(w, r, err)
		return
	}
	etag := "\"" + manifest.ArchiveDigest + "\""
	w.Header().Set("ETag", etag)
	if etagMatches(r.Header.Get("If-None-Match"), manifest.ArchiveDigest) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	writeJSON(w, http.StatusOK, manifest)
}

func etagMatches(header, digest string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		candidate = strings.TrimPrefix(candidate, "W/")
		if candidate == "*" || strings.Trim(candidate, "\"") == digest {
			return true
		}
	}
	return false
}

func (a *API) GetTimeline(w http.ResponseWriter, r *http.Request) {
	cursor, err := queryInt64(r, "cursor", 0)
	if err != nil {
		writeError(w, r, err)
		return
	}
	limit, err := queryInt64(r, "limit", 50)
	if err != nil {
		writeError(w, r, err)
		return
	}
	page, err := a.service.Timeline(r.Context(), r.PathValue("incident_id"), cursor, int(limit))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (a *API) VerifyArchive(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.VerifyArchive(r.Context(), r.PathValue("incident_id"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func queryInt64(r *http.Request, name string, defaultValue int64) (int64, error) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, preservation.Invalid(name, "必须为整数")
	}
	return parsed, nil
}
