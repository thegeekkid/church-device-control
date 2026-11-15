package main

import (
	"encoding/json"
	"testing"
)

func TestKasaInfoValidationAndModernNegotiation(t *testing.T) {
	for _, tc := range []struct {
		body              string
		wantError, legacy bool
	}{
		{`{"system":{"get_sysinfo":{"relay_state":0,"err_code":0}}}`, false, true},
		{`{"system":{"get_sysinfo":{"relay_state":1}}}`, false, true},
		{`{"error_code":-1008,"msg":"legacy API unsupported"}`, false, false},
		{`{"system":{"get_sysinfo":{"children":[]}}}`, false, false},
		{`{"system":{"get_sysinfo":{"relay_state":2}}}`, true, false},
		{`{"system":{"get_sysinfo":{"relay_state":-1}}}`, true, false},
		{`{"system":{"get_sysinfo":{"relay_state":"on"}}}`, true, false},
		{`{"system":{"get_sysinfo":{"relay_state":1,"err_code":-1,"err_msg":"failed"}}}`, true, false},
		{`{"system":{"get_sysinfo":{"relay_state":1,"alias":"Visible\u0000hidden"}}}`, true, false},
	} {
		var raw any
		if e := json.Unmarshal([]byte(tc.body), &raw); e != nil {
			t.Fatal(e)
		}
		d, e := legacyKasaInfo(raw)
		if (e != nil) != tc.wantError || (d != nil) != tc.legacy {
			t.Errorf("%s: legacy=%t error=%v", tc.body, d != nil, e)
		}
	}
}
