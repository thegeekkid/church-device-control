package main

import (
	"encoding/json"
	"reflect"
	"strings"
)

// Unknown additive device settings survive load/save, discovery, and sync.
// A new field must not require changing the envelope version.
type plainTV TV

var knownDeviceFields = func() map[string]bool {
	out := map[string]bool{}
	t := reflect.TypeOf(plainTV{})
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := strings.Split(f.Tag.Get("json"), ",")[0]
		if tag == "-" {
			continue
		}
		if tag == "" {
			tag = f.Name
		}
		out[strings.ToLower(tag)] = true
	}
	return out
}()

func (t TV) MarshalJSON() ([]byte, error) {
	b, e := json.Marshal(plainTV(t))
	if e != nil {
		return nil, e
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(b, &fields); e != nil {
		return nil, e
	}
	for k, v := range t.Extras {
		if !knownDeviceFields[strings.ToLower(k)] {
			fields[k] = v
		}
	}
	return json.Marshal(fields)
}
func (t *TV) UnmarshalJSON(b []byte) error {
	var p plainTV
	if e := json.Unmarshal(b, &p); e != nil {
		return e
	}
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(b, &fields); e != nil {
		return e
	}
	*t = TV(p)
	for k, v := range fields {
		if !knownDeviceFields[strings.ToLower(k)] {
			if t.Extras == nil {
				t.Extras = map[string]json.RawMessage{}
			}
			t.Extras[k] = v
		}
	}
	return nil
}
func mergeExtras(local, remote map[string]json.RawMessage, remoteWins bool) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for k, v := range local {
		out[k] = v
	}
	for k, v := range remote {
		if _, ok := out[k]; !ok || remoteWins {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
func disabledRevision(t TV) int64 {
	if t.DisabledUpdated != 0 {
		return t.DisabledUpdated
	}
	return t.Updated
}
