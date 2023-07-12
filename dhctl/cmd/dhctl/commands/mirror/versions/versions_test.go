package versions

import (
	"reflect"
	"testing"

	"github.com/Masterminds/semver/v3"
)

func Test_versionMajMinFromInt(t *testing.T) {
	type args struct {
		major uint64
		minor uint64
		patch uint64
	}
	tests := []struct {
		name string
		args args
		want *semver.Version
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := versionFromInt(tt.args.major, tt.args.minor, tt.args.patch); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("versionMajMinFromInt() = %v, want %v", got, tt.want)
			}
		})
	}
}
