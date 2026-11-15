package main

import (
	"encoding/json"
	"fmt"
)

const bridgeFeature = "v04-bridge-v1"

func hasCapability(c sharedConfig, name string) bool {
	for _, v := range c.Capabilities {
		if v == name {
			return true
		}
	}
	return false
}

// v0.4.0 merges whole records and can process a v1 compatibility view before
// the v2 feature view. Give the latter a one-nanosecond priority on that old
// reader only. Updated readers remove this transport-only priority; legacy
// unchanged echoes are ignored, so clocks do not grow on repeated exchanges.
func prepareV04Bridge(c *sharedConfig) {
	c.Capabilities = append(c.Capabilities, bridgeFeature)
	for i := range c.Devices {
		t := &c.Devices[i]
		t.Extras = mergeExtras(t.Extras, nil, false)
		if t.Extras == nil {
			t.Extras = map[string]json.RawMessage{}
		}
		b, _ := json.Marshal(t.Updated)
		t.Extras["CompatBaseUpdated"] = b
		t.Updated++
	}
}
func normalizeV04Bridge(c *sharedConfig) error {
	if c.Version != 2 || !hasCapability(*c, bridgeFeature) {
		return nil
	}
	for i := range c.Devices {
		t := &c.Devices[i]
		var base int64
		if e := json.Unmarshal(t.Extras["CompatBaseUpdated"], &base); e != nil || base < 0 || t.Updated != base+1 {
			return fmt.Errorf("invalid compatibility revision")
		}
		t.Updated = base
		delete(t.Extras, "CompatBaseUpdated")
		if len(t.Extras) == 0 {
			t.Extras = nil
		}
	}
	return nil
}
func sameCommonSettings(a, b TV) bool {
	a.Power = ""
	a.Status = ""
	a.Updated = 0
	a.Disabled = false
	a.DisabledUpdated = 0
	a.Extras = nil
	b.Power = ""
	b.Status = ""
	b.Updated = 0
	b.Disabled = false
	b.DisabledUpdated = 0
	b.Extras = nil
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
func sameExtras(a, b map[string]json.RawMessage) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y) || (len(a) == 0 && len(b) == 0)
}
