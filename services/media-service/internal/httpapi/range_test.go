package httpapi

import "testing"

func TestParseByteRange(t *testing.T) {
	tests := []struct {
		name        string
		header      string
		size        int64
		want        byteRange
		wantPresent bool
		wantError   bool
	}{
		{name: "no range", size: 100},
		{name: "bounded", header: "bytes=10-19", size: 100,
			want: byteRange{start: 10, end: 19}, wantPresent: true},
		{name: "open ended", header: "bytes=90-", size: 100,
			want: byteRange{start: 90, end: 99}, wantPresent: true},
		{name: "suffix", header: "bytes=-10", size: 100,
			want: byteRange{start: 90, end: 99}, wantPresent: true},
		{name: "suffix larger than object", header: "bytes=-200", size: 100,
			want: byteRange{start: 0, end: 99}, wantPresent: true},
		{name: "end clamped", header: "bytes=95-200", size: 100,
			want: byteRange{start: 95, end: 99}, wantPresent: true},
		{name: "start outside object", header: "bytes=100-", size: 100,
			wantPresent: true, wantError: true},
		{name: "reversed", header: "bytes=20-10", size: 100,
			wantPresent: true, wantError: true},
		{name: "zero suffix", header: "bytes=-0", size: 100,
			wantPresent: true, wantError: true},
		{name: "multiple", header: "bytes=0-1,4-5", size: 100,
			wantPresent: true, wantError: true},
		{name: "wrong unit", header: "items=0-1", size: 100,
			wantPresent: true, wantError: true},
		{name: "empty object", header: "bytes=0-0", size: 0,
			wantPresent: true, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, present, err := parseByteRange(test.header, test.size)
			if present != test.wantPresent || (err != nil) != test.wantError || got != test.want {
				t.Fatalf("parseByteRange(%q, %d) = %+v, %t, %v; want %+v, %t, error=%t",
					test.header, test.size, got, present, err,
					test.want, test.wantPresent, test.wantError)
			}
		})
	}
}
