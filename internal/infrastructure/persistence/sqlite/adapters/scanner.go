package adapters

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

type JSONStringSlice []string

func (s JSONStringSlice) Value() (driver.Value, error) {
	if len(s) == 0 {
		return "[]", nil
	}
	bytes, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("marshal string slice: %w", err)
	}
	return string(bytes), nil
}

func (s *JSONStringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = []string{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		str, ok := value.(string)
		if !ok {
			return errors.New("failed to scan JSONStringSlice: invalid type")
		}
		bytes = []byte(str)
	}

	var result []string
	if err := json.Unmarshal(bytes, &result); err != nil {
		return fmt.Errorf("unmarshal JSONStringSlice: %w", err)
	}

	*s = result
	return nil
}
