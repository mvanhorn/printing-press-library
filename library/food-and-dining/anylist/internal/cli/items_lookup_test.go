// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
)

func TestFormatPackageSize(t *testing.T) {
	tests := []struct {
		name string
		in   *pb.PBItemPackageSize
		want string
	}{
		{name: "raw", in: &pb.PBItemPackageSize{RawPackageSize: "12 count, 12 fl oz"}, want: "12 count, 12 fl oz"},
		{name: "structured", in: &pb.PBItemPackageSize{Size: "16", Unit: "oz", PackageType: "jar"}, want: "16 oz jar"},
		{name: "nil", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatPackageSize(tt.in); got != tt.want {
				t.Fatalf("formatPackageSize() = %q, want %q", got, tt.want)
			}
		})
	}
}
