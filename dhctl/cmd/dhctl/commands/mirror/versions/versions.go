package versions

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/Masterminds/semver/v3"
)

var (
	ErrNoVersion = errors.New("no such version")
)

func Parse(v string) (*semver.Version, error) {
	return semver.NewVersion(v)
}

func ParseFromInt(major, minor, patch uint64) *semver.Version {
	return versionFromInt(major, minor, patch)
}

type LatestVersions map[semver.Version]semver.Version /* map[<maj>.<min>]max(<maj>.<min>.<patch>) */

func (vs LatestVersions) SetString(v string) (bool, error) {
	versionWithPatch, err := Parse(v)
	if err != nil {
		return false, err
	}
	return vs.Set(*versionWithPatch)
}

func (vs LatestVersions) Set(new semver.Version) (bool, error) {
	old, err := vs.Get(new)
	switch {
	case errors.Is(err, ErrNoVersion):
	case err != nil:
		return false, err
	case old.GreaterThan(&new):
		return false, nil
	}

	vs[prepareKey(new)] = new
	return true, nil
}

func (vs LatestVersions) GetString(v string) (*semver.Version, error) {
	key, err := Parse(v)
	if err != nil {
		return nil, err
	}
	return vs.Get(*key)
}

func (vs LatestVersions) Get(key semver.Version) (*semver.Version, error) {
	v, ok := vs[prepareKey(key)]
	if !ok {
		return nil, ErrNoVersion
	}
	return &v, nil
}

func (vs LatestVersions) Latest() *semver.Version {
	var maxValue semver.Version
	for _, value := range vs {
		if value.GreaterThan(&maxValue) {
			maxValue = value
		}
	}
	return &maxValue
}

func (vs LatestVersions) Oldest() *semver.Version {
	minValue := *versionFromInt(100000, 0, 0)
	for _, value := range vs {
		if minValue.GreaterThan(&value) {
			minValue = value
		}
	}
	return &minValue
}

func prepareKey(key semver.Version) semver.Version {
	return *versionFromInt(key.Major(), key.Minor(), 0)
}

func versionFromInt(major, minor, patch uint64) *semver.Version {
	version := fmt.Sprintf("v%d.%d", major, minor)
	if patch > 0 {
		version = version + "." + strconv.FormatUint(patch, 10)
	}
	v, _ := semver.NewVersion(version)
	return v
}
