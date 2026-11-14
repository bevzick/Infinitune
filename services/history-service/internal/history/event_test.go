package history

import "testing"

func TestQualifies(t *testing.T) {
	for _, test := range []struct {
		name               string
		listened, duration int
		want               bool
	}{
		{"long track at threshold", 30000, 180000, true},
		{"long track below threshold", 29999, 180000, false},
		{"short track at half", 10000, 20000, true},
		{"invalid duration", 10000, 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := Qualifies(test.listened, test.duration); got != test.want {
				t.Fatalf("Qualifies() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestIsCompleted(t *testing.T) {
	if !IsCompleted(90000, 100000) {
		t.Fatal("90 percent should be completed")
	}
	if IsCompleted(89999, 100000) {
		t.Fatal("less than 90 percent should not be completed")
	}
}
