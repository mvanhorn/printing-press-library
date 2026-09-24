package zimmo

import (
	"reflect"
	"testing"
)

// The search API rejects the whole request when one energyLabel value is not
// in its enumeration; F and G have no +/- variants.
func TestExpandEPCOnlyEmitsAcceptedLabels(t *testing.T) {
	if got := expandEPC([]string{"F", "G"}); !reflect.DeepEqual(got, []string{"F", "G"}) {
		t.Errorf("F,G = %v", got)
	}
	if got := expandEPC([]string{"D"}); !reflect.DeepEqual(got, []string{"D", "D_PLUS", "D_MINUS"}) {
		t.Errorf("D = %v", got)
	}
	if got := expandEPC([]string{"A"}); len(got) != 4 {
		t.Errorf("A = %v", got)
	}
	for _, v := range expandEPC([]string{"A", "B", "C", "D", "E", "F", "G"}) {
		if !zimmoEnergyLabels[v] {
			t.Errorf("%s is not an accepted label", v)
		}
	}
}
