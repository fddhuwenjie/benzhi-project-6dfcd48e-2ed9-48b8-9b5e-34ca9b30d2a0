package preservation

import "fmt"

type ErrorCode string

const (
	CodeValidation ErrorCode = "validation_failed"
	CodeState      ErrorCode = "invalid_state"
	CodeSealed     ErrorCode = "incident_sealed"
	CodeNotFound   ErrorCode = "not_found"
	CodeConflict   ErrorCode = "revision_conflict"
	CodeIntegrity  ErrorCode = "integrity_failure"
)

type DomainError struct {
	Code            ErrorCode `json:"code"`
	Message         string    `json:"message"`
	CurrentRevision int64     `json:"current_revision,omitempty"`
	ActualDigest    string    `json:"actual_digest,omitempty"`
}

func (e *DomainError) Error() string { return e.Message }

func Invalid(field, reason string) error {
	return &DomainError{Code: CodeValidation, Message: fmt.Sprintf("%s: %s", field, reason)}
}

func WrongState(current IncidentState, allowed ...IncidentState) error {
	return &DomainError{Code: CodeState, Message: fmt.Sprintf("当前状态 %s 不允许此操作，允许状态为 %v", current, allowed)}
}

func EnsureMutable(i *PreservationIncident) error {
	if i.State == StateSealed || i.SealedAt != nil {
		return &DomainError{Code: CodeSealed, Message: "事件已经封存，禁止继续修改"}
	}
	return nil
}
