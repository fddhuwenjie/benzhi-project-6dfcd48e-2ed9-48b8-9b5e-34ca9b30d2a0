package preservation

import (
	"regexp"
	"strings"
	"time"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{1,127}$`)

func ValidateIdentifier(field, value string) error {
	if !identifierPattern.MatchString(value) {
		return Invalid(field, "必须为 2 至 128 位安全标识符")
	}
	return nil
}

func ValidateActor(actor string) error { return ValidateIdentifier("actor_id", actor) }

func ValidateTimestamp(field string, value time.Time) error {
	if value.IsZero() {
		return Invalid(field, "不能为空")
	}
	if value.After(time.Now().UTC().Add(10 * time.Minute)) {
		return Invalid(field, "不能显著晚于当前时间")
	}
	return nil
}

func requireText(field, value string, max int) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return Invalid(field, "不能为空")
	}
	if len(value) > max {
		return Invalid(field, "长度超出限制")
	}
	return nil
}

func uniqueNonEmpty(field string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if err := ValidateIdentifier(field, value); err != nil {
			return err
		}
		if _, ok := seen[value]; ok {
			return Invalid(field, "不能包含重复值")
		}
		seen[value] = struct{}{}
	}
	return nil
}
