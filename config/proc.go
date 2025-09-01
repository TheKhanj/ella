package config

import (
	"encoding/json"
	"errors"
	"fmt"
)

func (this *Proc) GetStop() (StopProcAction, error) {
	stop := this.Stop
	if stopSignal, ok := stop.(string); ok {
		return ProcActionSignalCode(stopSignal), nil
	} else if m, ok := stop.(map[string]any); ok {
		switch m["type"] {
		case "signal":
			var c StopProcActionSignal
			b, err := json.Marshal(stop)
			if err != nil {
				return nil, err
			}
			err = c.UnmarshalJSON(b)
			if err != nil {
				return nil, err
			}
			return &c, nil
		case "exec":
			return nil, errors.New("not implemented")
		default:
			return nil, fmt.Errorf("invalid action type: %s", m["type"])
		}
	} else {
		return nil, fmt.Errorf("invalid stop action: %v", stop)
	}
}

func (this *Proc) GetReload() (ReloadProcAction, error) {
	reload := this.Reload
	if reloadSignal, ok := reload.(string); ok {
		return ProcActionSignalCode(reloadSignal), nil
	} else if m, ok := reload.(map[string]any); ok {
		switch m["type"] {
		case "exec":
			return nil, errors.New("not implemented")
		default:
			return nil, fmt.Errorf("invalid action type: %s", m["type"])
		}
	} else {
		return nil, fmt.Errorf("invalid reload action: %v", reload)
	}
}

func (this *Proc) GetWatchdog() (ProcWatchdog, error) {
	var m map[string]any
	m, ok := this.Watchdog.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid watchdog config: %v", this.Watchdog)
	}

	switch m["strategy"] {
	case "simple":
		return &SimpleWatchdog{
			Strategy: "simple",
		}, nil
	default:
		return nil, fmt.Errorf("invalid watchdog strategy: %s", m["strategy"])
	}
}
