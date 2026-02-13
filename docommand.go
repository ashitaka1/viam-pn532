package pn532

import (
	"context"
	"fmt"
)

func (s *pn532Sensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	action, ok := cmd["action"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid \"action\" field in command")
	}

	return nil, fmt.Errorf("action %q is not yet implemented", action)
}
