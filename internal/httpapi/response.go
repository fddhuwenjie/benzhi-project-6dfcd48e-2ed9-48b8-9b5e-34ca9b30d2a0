package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"

	"tape-preservation-incident-api/internal/preservation"
)

type protocolError struct {
	Error struct {
		Code            string `json:"code"`
		Message         string `json:"message"`
		CorrelationID   string `json:"correlation_id"`
		CurrentRevision int64  `json:"current_revision,omitempty"`
		ActualDigest    string `json:"actual_digest,omitempty"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	message := "服务暂时无法完成请求"
	currentRevision := int64(0)
	var domain *preservation.DomainError
	if errors.As(err, &domain) {
		code, message, currentRevision = string(domain.Code), domain.Message, domain.CurrentRevision
		switch domain.Code {
		case preservation.CodeValidation:
			status = http.StatusBadRequest
		case preservation.CodeNotFound:
			status = http.StatusNotFound
		case preservation.CodeConflict, preservation.CodeState, preservation.CodeSealed:
			status = http.StatusConflict
		case preservation.CodeIntegrity:
			status = http.StatusInternalServerError
		}
	}
	response := protocolError{}
	response.Error.Code = code
	response.Error.Message = message
	response.Error.CorrelationID = correlationID(r)
	response.Error.CurrentRevision = currentRevision
	if domain != nil {
		response.Error.ActualDigest = domain.ActualDigest
	}
	writeJSON(w, status, response)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return preservation.Invalid("Content-Type", "必须为 application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return preservation.Invalid("body", "请求体超过 1 MiB")
		}
		return preservation.Invalid("body", fmt.Sprintf("JSON 无效: %v", err))
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return preservation.Invalid("body", "只能包含一个 JSON 对象")
	}
	return nil
}
