package preservation

import "encoding/json"

func Clone(incident *PreservationIncident) (*PreservationIncident, error) {
	data, err := json.Marshal(incident)
	if err != nil {
		return nil, err
	}
	var cloned PreservationIncident
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}
