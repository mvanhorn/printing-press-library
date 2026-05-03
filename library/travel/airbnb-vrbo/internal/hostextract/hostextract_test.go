package hostextract

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-vrbo/internal/source/vrbo"
)

func TestFromVRBOProperty(t *testing.T) {
	tests := []struct {
		name string
		in   *vrbo.Property
		want string
	}{
		{name: "happy path", in: &vrbo.Property{Title: "Near Ski Resorts | Basque Lodge by AvantStay"}, want: "AvantStay"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromVRBOProperty(tt.in)
			if got == nil || got.Brand != tt.want {
				t.Fatalf("Brand = %#v, want %q", got, tt.want)
			}
		})
	}
}
