package versions

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/containers/image/v5/signature"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/image"
)

const (
	VersionRE = `(v[0-9]+\.[0-9]+)\.[0-9]+`
)

var (
	versionsRegexp  = regexp.MustCompile(`^` + VersionRE + `$`)
	releaseChannels = []string{"alpha", "beta", "early-access", "stable", "rock-solid"}
)

type VersionsComparer struct {
	source *image.RegistryConfig
	dest   *image.RegistryConfig

	policyContext *signature.PolicyContext

	destListOpts   []image.ListOption
	sourceListOpts []image.ListOption

	sourceCopyOpts []image.CopyOption
}

func NewVersionsComparer(source, dest *image.RegistryConfig, destListOpts, sourceListOpts []image.ListOption, sourceCopyOpts []image.CopyOption, policyContext *signature.PolicyContext) *VersionsComparer {
	return &VersionsComparer{
		source:         source,
		dest:           dest,
		sourceListOpts: sourceListOpts,
		destListOpts:   destListOpts,
		sourceCopyOpts: sourceCopyOpts,
		policyContext:  policyContext,
	}
}
func (v *VersionsComparer) ImagesToCopy(ctx context.Context, minVersion string) ([]*image.ImageConfig, error) {
	diff, err := v.calculateDiff(ctx, minVersion)
	if err != nil {
		return nil, err
	}
	fmt.Println(diff)
	modulesImages := make(map[semver.Version]map[string]string)
	for _, tag := range diff {
		versionImages, err := v.modulesImages(ctx, tag)
		if err != nil {
			return nil, err
		}
		modulesImages[tag] = versionImages
	}

	type imageSpec struct {
		imageName string
		d8Version semver.Version
	}
	uniqueImages := make(map[string]imageSpec)
	for version, versionImages := range modulesImages {
		for imageName, identifier := range versionImages {
			if old, ok := uniqueImages[identifier]; ok && version.GreaterThan(&old.d8Version) {
				continue
			}
			uniqueImages[identifier] = imageSpec{
				imageName: imageName,
				d8Version: version,
			}
		}
	}

	images := make([]*image.ImageConfig, 0, len(uniqueImages)+len(diff)*2+len(releaseChannels)*3+1)
	for identifier, imgSpec := range uniqueImages {
		tag, digest := identifier, ""
		if strings.HasPrefix(identifier, "sha256:") {
			tag, digest = versionToTag(imgSpec.d8Version)+"."+imgSpec.imageName, identifier
		}
		images = append(images, image.NewImageConfig(v.source, tag, digest))
	}

	for _, version := range diff {
		images = append(images, image.NewImageConfig(v.source, versionToTag(version), ""), image.NewImageConfig(v.source, versionToTag(version), "install"))
	}

	for _, release := range releaseChannels {
		images = append(images, image.NewImageConfig(v.source, release, ""), image.NewImageConfig(v.source, release, "", "install"), image.NewImageConfig(v.source, release, "", "release-channel"))
	}
	images = append(images, image.NewImageConfig(v.source, "2", "", "security", "trivy-db"))
	return images, nil
}

func (v *VersionsComparer) modulesImages(ctx context.Context, deckhouseVersion semver.Version) (map[string]string, error) {
	img := image.NewImageConfig(v.source, versionToTag(deckhouseVersion), "")
	contents, err := fileFromImage(ctx, img, "deckhouse/modules/images_digests.json", v.policyContext, v.sourceCopyOpts...)
	if err != nil {
		contents, err = fileFromImage(ctx, img, "deckhouse/modules/images_tags.json", v.policyContext, v.sourceCopyOpts...)
		if err != nil {
			return nil, err
		}
	}

	var modulesImages map[string]map[string]string
	if err := json.Unmarshal(contents, &modulesImages); err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for module, images := range modulesImages {
		for image, identifier := range images {
			result[module+"."+image] = identifier
		}
	}
	return result, nil
}

func (v *VersionsComparer) calculateDiff(ctx context.Context, minVersion string) ([]semver.Version, error) {
	sourceVersions, err := v.sourceVersions(ctx)
	if err != nil {
		return nil, err
	}

	destVersions, err := v.destVersions(ctx, sourceVersions, minVersion)
	if err != nil {
		return nil, err
	}

	deckhouseVersions, err := compareVersions(sourceVersions, destVersions)
	if err != nil {
		return nil, err
	}

	releaseMetaVersions, err := v.releaseMetadataVersions(ctx, destVersions)
	if err != nil {
		return nil, err
	}

	result := make([]semver.Version, 0, len(deckhouseVersions)+len(releaseMetaVersions))
	for _, v := range deckhouseVersions {
		if f := releaseMetaVersions[v]; !f {
			result = append(result, v)
		}
	}
	for v := range releaseMetaVersions {
		result = append(result, v)
	}

	return result, nil
}

func (v *VersionsComparer) sourceVersions(ctx context.Context) (latestVersions, error) {
	return findDeckhouseVersions(ctx, v.source, v.sourceListOpts...)
}

func (v *VersionsComparer) destVersions(ctx context.Context, sourceVersions latestVersions, minVersion string) (latestVersions, error) {
	destVersions := make(latestVersions)
	if v.dest.Transport() == image.DockerTransport {
		var err error
		destVersions, err = findDeckhouseVersions(ctx, v.dest, v.destListOpts...)
		if err != nil {
			return nil, err
		}
	}

	if len(destVersions) < 1 || v.dest.Transport() != image.DockerTransport {
		minVersion, err := deckhouseMinVersion(sourceVersions, minVersion)
		if err != nil {
			return nil, fmt.Errorf("min version: %w", err)
		}
		if _, err := destVersions.SetString(minVersion.String()); err != nil {
			return nil, err
		}
	}
	return destVersions, nil
}

func (v *VersionsComparer) releaseMetadataVersions(ctx context.Context, destVersions latestVersions) (map[semver.Version]bool, error) {
	releaseMetaVersions := make(map[semver.Version]bool, len(releaseChannels))
	for _, release := range releaseChannels {
		releaseVersion, err := v.fetchReleaseMetadataDeckhouseVersion(ctx, release)
		if err != nil {
			return nil, err
		}

		dv, err := destVersions.Get(*releaseVersion)
		if err != nil && !errors.Is(err, ErrNoVersion) {
			return nil, err
		}

		if errors.Is(err, ErrNoVersion) || !dv.Equal(releaseVersion) {
			releaseMetaVersions[*releaseVersion] = true
		}
	}
	return releaseMetaVersions, nil
}

// fetchReleaseMetadataDeckhouseVersion copies image to local directory and untar it's layers to find version.json and returns "version" key found in it
func (v *VersionsComparer) fetchReleaseMetadataDeckhouseVersion(ctx context.Context, release string) (*semver.Version, error) {
	img := image.NewImageConfig(v.source, release, "", "release-channel")
	contents, err := fileFromImage(ctx, img, "version.json", v.policyContext, v.sourceCopyOpts...)
	if err != nil {
		return nil, err
	}

	var meta struct {
		Version string `json:"version"`
	}

	if err := json.Unmarshal(contents, &meta); err != nil {
		return nil, err
	}
	return parse(meta.Version)
}

func compareVersions(sourceVersions, destVersions latestVersions) (latestVersions, error) {
	sourceLatestWithPatch, destOldestWithPatch := sourceVersions.Latest(), destVersions.Oldest()
	resultVersions := make(latestVersions)
	for version := *parseFromInt(destOldestWithPatch.Major(), destOldestWithPatch.Minor(), 0); !version.GreaterThan(sourceLatestWithPatch); version = version.IncMinor() {
		sourceVersion, err := sourceVersions.Get(version)
		if err != nil {
			return nil, fmt.Errorf("version %s from source: %w", version, err)
		}

		destVersion, err := destVersions.Get(version)
		switch {
		case (err == nil && !destVersion.Equal(sourceVersion)) || errors.Is(err, ErrNoVersion):
			if _, err := resultVersions.Set(*sourceVersion); err != nil {
				return nil, err
			}
		case err != nil:
			return nil, fmt.Errorf("version %s from destination: %w", version, err)
		}
	}
	return resultVersions, nil
}

func findDeckhouseVersions(ctx context.Context, registry *image.RegistryConfig, opts ...image.ListOption) (latestVersions, error) {
	tags, err := registry.ListTags(ctx, opts...)
	if err != nil {
		return nil, err
	}

	versions := make(latestVersions)
	for _, tag := range tags {
		if !versionsRegexp.MatchString(tag) {
			continue
		}

		if _, err := versions.SetString(tag); err != nil {
			return nil, err
		}
	}
	return versions, nil
}

func deckhouseMinVersion(sourceVersions latestVersions, minVersion string) (*semver.Version, error) {
	latestWithPatch := sourceVersions.Latest()
	switch minVersion {
	case "":
		version := parseFromInt(latestWithPatch.Major(), latestWithPatch.Minor()-5, 0)
		return sourceVersions.Get(*version)
	case "latest":
		return latestWithPatch, nil
	}
	return sourceVersions.GetString(minVersion)
}

func fileFromImage(ctx context.Context, img *image.ImageConfig, filename string, policyContext *signature.PolicyContext, opts ...image.CopyOption) ([]byte, error) {
	dir, err := os.MkdirTemp("/tmp", "deckhouse_*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	destRegistry, err := image.NewRegistry("dir:"+dir, nil)
	if err != nil {
		return nil, err
	}

	dest := img.WithNewRegistry(destRegistry)
	if err := image.CopyImage(ctx, img, dest, policyContext, opts...); err != nil {
		return nil, err
	}

	imageDir := filepath.Join(dest.RegistryPath(), img.Tag())
	files, err := os.ReadDir(imageDir)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		contents, err := fileFromTarGz(filepath.Join(imageDir, file.Name()), filename)
		if errors.Is(err, io.EOF) || errors.Is(err, tar.ErrHeader) || errors.Is(err, gzip.ErrHeader) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return contents, nil
	}
	return nil, fmt.Errorf(`"%s" file not found in image from "%s" dir`, filename, dir)
}

func versionToTag(v semver.Version) string {
	return "v" + v.String()
}

// fileFromGzTarLayer finds finds "targetFile" in "archive" tar.gz file
func fileFromTarGz(archive, targetFile string) ([]byte, error) {
	file, err := os.Open(archive)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err != nil {
			return nil, err
		}

		if hdr.Name != targetFile {
			continue
		}

		buf := bytes.NewBuffer(nil)
		_, err = io.Copy(buf, tr)
		return buf.Bytes(), err
	}
}
