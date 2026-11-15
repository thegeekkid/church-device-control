package main

import "fmt"

type kasaInfo struct {
	Alias, Model, DeviceID, MAC string
	Relay                       *int   `json:"relay_state"`
	Code                        *int   `json:"err_code"`
	Message                     string `json:"err_msg"`
}

// tapo bounds network frames and authenticates KLAP in its transport. This
// validates the JSON types and single-plug relay semantics used by our adapter.
// A nil result selects the existing modern/strip API path, rather than treating
// an unrelated modern response as a valid legacy relay response.
func legacyKasaInfo(raw any) (*kasaInfo, error) {
	var r struct {
		Code    *int   `json:"error_code"`
		Message string `json:"msg"`
		System  struct {
			Info *kasaInfo `json:"get_sysinfo"`
		}
	}
	if e := decodeKasa(raw, &r); e != nil {
		return nil, fmt.Errorf("invalid Kasa device response: %w", e)
	}
	d := r.System.Info
	if d == nil {
		// Modern plugs reject get_sysinfo with a top-level error_code. Let the
		// existing Info/Outlets path negotiate their API and validate its errors.
		return nil, nil
	}
	if r.Code != nil && *r.Code != 0 {
		return nil, fmt.Errorf("Kasa device error %d: %s", *r.Code, r.Message)
	}
	if d.Code != nil && *d.Code != 0 {
		return nil, fmt.Errorf("Kasa system info error %d: %s", *d.Code, d.Message)
	}
	if d.Relay == nil {
		return nil, nil
	}
	if *d.Relay != 0 && *d.Relay != 1 {
		return nil, fmt.Errorf("invalid Kasa relay state: %d", *d.Relay)
	}
	// Some firmware omits a success err_code; retain that compatibility while
	// rejecting explicit errors and malformed types. Commands still require a
	// success err_code in relayCommandResult before they are marked accepted.
	if e := validateDeviceMetadata(TV{Name: d.Alias, Model: d.Model, Serial: d.DeviceID, MAC: d.MAC}); e != nil {
		return nil, e
	}
	return d, nil
}
