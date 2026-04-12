package api

import "testing"

func TestResolveIdentifier(t *testing.T) {
	tests := []struct {
		headcode  string
		tiploc    string
		trainuid  string
		wantType  string
		wantValue string
	}{
		{"1A23", "", "", "headcode", "1A23"},
		{"", "DRBY", "", "tiploc", "DRBY"},
		{"", "", "C00001", "trainuid", "C00001"},
		{"", "", "", "headcode", ""},
		// headcode takes precedence over tiploc and trainuid
		{"1A23", "DRBY", "C00001", "headcode", "1A23"},
		// tiploc takes precedence over trainuid
		{"", "DRBY", "C00001", "tiploc", "DRBY"},
	}

	for _, tt := range tests {
		identType, ident := resolveIdentifier(tt.headcode, tt.tiploc, tt.trainuid)
		if identType != tt.wantType || ident != tt.wantValue {
			t.Errorf("resolveIdentifier(%q, %q, %q) = (%q, %q), want (%q, %q)",
				tt.headcode, tt.tiploc, tt.trainuid,
				identType, ident,
				tt.wantType, tt.wantValue)
		}
	}
}
