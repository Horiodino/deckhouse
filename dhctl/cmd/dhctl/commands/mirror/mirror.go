// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mirror

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/Masterminds/semver/v3"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/image"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/versions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	eeEdition = "ee"

	destinationHelp = `destination for images to write (archive file: "file:<file path>.tar.gz" or registry: "docker://<registry repositroy").`
	sourceHelp      = `source for deckhouse images (archive file: "file:<file path>.tar.gz" or registry: "docker://<registry repositroy").`

	registryRegexp = "^(file:.+\\.tar\\.gz|docker://.+)$"

	releaseChannelRepo = "release-channel"
	versionRE          = `(v[0-9]+\.[0-9]+)\.[0-9]+`
)

var (
	releaseChannels = [5]string{"alpha", "beta", "early-access", "stable", "rock-solid"}

	ErrLatestLowerThanMin = errors.New("latest version can't be lower than min version")
	ErrEditionNotEE       = errors.New("dhctl mirror can be used only in EE deckhouse edition")
	ErrNoLicense          = errors.New("license is required to download Deckhouse Enterprise Edition. Please provide it with CLI argument --license")

	versionLatestRE = fmt.Sprintf(`^(%s|latest)$`, versionRE)
	versionsRegexp  = regexp.MustCompile(`^` + versionRE + `$`)
)

func DefineMirrorCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	var (
		minVersion = app.NewStringWithRegexpValidation(versionLatestRE)

		source       = app.NewStringWithRegexpValidation(registryRegexp)
		licenseToken string

		destination         = app.NewStringWithRegexpValidation(registryRegexp)
		destinationUser     string
		destinationPassword string
		destinationInsecure bool
		dryRun              bool
	)

	cmd := kpApp.Command("mirror", "Copy images from deckhouse registry or tar.gz file to specified registry or tar.gz file.")

	cmd.Arg("DESTINATION", destinationHelp).Required().SetValue(destination)
	cmd.Flag("from", sourceHelp).Default("docker://registry.deckhouse.io/deckhouse").SetValue(source)

	cmd.Flag("dry-run", "Run without actually copying data.").BoolVar(&dryRun)
	cmd.Flag("min-version", `The oldest version of deckhouse from your clusters or "latest" for clean installation.`).SetValue(minVersion)

	// Deckhouse registry flags
	cmd.Flag("license", "License key for Deckhouse registry.").Required().StringVar(&licenseToken)

	// Destination registry flags
	cmd.Flag("username", "Username for the destination registry.").StringVar(&destinationUser)
	cmd.Flag("password", "Password for the destination registry.").StringVar(&destinationPassword)
	cmd.Flag("insecure", "Use http instead of https while connecting to destination registry.").BoolVar(&destinationInsecure)

	runFunc := func() error {
		ctx := context.Background()

		edition, err := deckhouseEdition()
		if err != nil {
			return err
		}

		source, err := deckhouseRegistry(source.String(), edition, licenseToken)
		if err != nil {
			return err
		}

		destination, err := newRegistry(destination.String(), registryAuth(destinationUser, destinationPassword))
		if err != nil {
			return err
		}

		destListOptions := make([]image.ListOption, 0)
		if destinationInsecure {
			destListOptions = append(destListOptions, image.WithInsecure())
		}

		versions, err := versionsToCopy(ctx, source, destination, minVersion.String(), destListOptions...)
		if err != nil {
			return err
		}
		fmt.Println(versions)
		// latestVersion, err := deckhouseLatestVersion(ctx, source)
		// if err != nil {
		// 	return err
		// }

		// minVersion, err := deckhouseMinVersion(latestVersion, minVersion.String())
		// if err != nil {
		// 	return err
		// }

		// versions, err := deckhouseVersions(minVersion, latestVersion)
		// if err != nil {
		// 	return err
		// }

		// fmt.Println(versions)

		// fmt.Println(edition)

		// fmt.Println(source, destination)

		// listOptions := make([]image.ListOption, 0)
		// if destinationInsecure {
		// 	listOptions = append(listOptions, image.WithInsecure())
		// }

		// allVersions, err := excludeDestinationVersions(ctx, destination, versions, listOptions...)
		// if err != nil {
		// 	return err
		// }
		// fmt.Println(allVersions)

		policyContext, err := image.NewPolicyContext()
		if err != nil {
			return err
		}
		defer policyContext.Destroy()

		// copyOpts := []image.CopyOption{
		// 	image.WithCopyAllImages(),
		// 	image.WithPreserveDigests(),
		// 	image.WithOutput(log.GetDefaultLogger()),
		// }

		// if destinationInsecure {
		// 	copyOpts = append(copyOpts, image.WithDestInsecure())
		// }

		// images, err := deckhouseImages(ctx, source.String(), edition, version, licenseToken, policyContext, copyOpts...)
		// if err != nil {
		// 	return err
		// }

		// destRegistry, err := image.NewRegistry(destination.String(), registryAuth(destinationUser, destinationPassword))
		// if err != nil {
		// 	return err
		// }

		// if dryRun {
		// 	copyOpts = append(copyOpts, image.WithDryRun())
		// }

		// // Copy images
		// for _, srcImage := range images {
		// 	if err := copyImage(ctx, srcImage, destRegistry, policyContext, copyOpts...); err != nil {
		// 		return err
		// 	}
		// }
		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		return log.Process("mirror", "Copy images", runFunc)
	})
	return cmd
}

func deckhouseEdition() (string, error) {
	content, err := os.ReadFile("/deckhouse/edition")
	if err != nil {
		return "", err
	}

	edition := strings.TrimSpace(string(content))
	if edition != eeEdition {
		return "", ErrEditionNotEE
	}

	return edition, nil
}

func deckhouseRegistry(deckhouseRegistry, edtiton, licenseToken string) (*image.RegistryConfig, error) {
	registry, err := newRegistry(deckhouseRegistry, nil)
	if err != nil {
		return nil, err
	}

	if registry.Transport() != image.DockerTransport {
		return registry, nil
	}

	auth, err := deckhouseRegistryAuth(edtiton, licenseToken)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(deckhouseRegistry)
	if err != nil {
		return nil, err
	}
	u.Path = filepath.Join(u.Path, edtiton)
	return newRegistry(u.String(), auth)
}

func deckhouseRegistryAuth(edition, licenseToken string) (*types.DockerAuthConfig, error) {
	if licenseToken == "" {
		return nil, ErrNoLicense
	}
	return registryAuth("license-token", licenseToken), nil
}

func newRegistry(registryWithTransport string, auth *types.DockerAuthConfig) (*image.RegistryConfig, error) {
	return image.NewRegistry(registryWithTransport, auth)
}

func registryAuth(username, password string) *types.DockerAuthConfig {
	if username == "" || password == "" {
		return nil
	}

	return &types.DockerAuthConfig{
		Username: username,
		Password: password,
	}
}

func versionsToCopy(ctx context.Context, source, dest *image.RegistryConfig, minVersion string, destListOpts ...image.ListOption) (versions.LatestVersions, error) {
	sourceVersions, err := findDeckhouseVersions(ctx, source)
	if err != nil {
		return nil, err
	}

	destVersions := make(versions.LatestVersions)
	if dest.Transport() == image.DockerTransport {
		var err error
		destVersions, err = findDeckhouseVersions(ctx, dest, destListOpts...)
		if err != nil {
			return nil, err
		}
	}

	if len(destVersions) < 1 || dest.Transport() != image.DockerTransport {
		minVersion, err := deckhouseMinVersion(sourceVersions, minVersion)
		if err != nil {
			return nil, fmt.Errorf("min version: %w", err)
		}
		if _, err := destVersions.SetString(minVersion.String()); err != nil {
			return nil, err
		}
	}

	return compareDeckhouseVersions(sourceVersions, destVersions)
}

func findDeckhouseVersions(ctx context.Context, registry *image.RegistryConfig, opts ...image.ListOption) (versions.LatestVersions, error) {
	tags, err := registry.ListTags(ctx, opts...)
	if err != nil {
		return nil, err
	}

	versions := make(versions.LatestVersions)
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

func deckhouseMinVersion(sourceVersions versions.LatestVersions, minVersion string) (*semver.Version, error) {
	latestWithPatch := sourceVersions.Latest()
	switch minVersion {
	case "":
		version := versions.ParseFromInt(latestWithPatch.Major(), latestWithPatch.Minor()-5, 0)
		return sourceVersions.Get(*version)
	case "latest":
		return latestWithPatch, nil
	}
	return sourceVersions.GetString(minVersion)
}

func compareDeckhouseVersions(sourceVersions, destVersions versions.LatestVersions) (versions.LatestVersions, error) {
	sourceLatestWithPatch, destOldestWithPatch := sourceVersions.Latest(), destVersions.Oldest()
	resultVersions := make(versions.LatestVersions)
	for version := *versions.ParseFromInt(destOldestWithPatch.Major(), destOldestWithPatch.Minor(), 0); !version.GreaterThan(sourceLatestWithPatch); version = version.IncMinor() {
		sourceVersion, err := sourceVersions.Get(version)
		if err != nil {
			return nil, fmt.Errorf("version %s from source: %w", version, err)
		}

		destVersion, err := destVersions.Get(version)
		switch {
		case (err == nil && !destVersion.Equal(sourceVersion)) || errors.Is(err, versions.ErrNoVersion):
			if _, err := resultVersions.Set(*sourceVersion); err != nil {
				return nil, err
			}
		case err != nil:
			return nil, fmt.Errorf("version %s from destination: %w", version, err)
		}
	}
	return resultVersions, nil
}

// func generateImagesList(source, destination *image.RegistryConfig, versions []semver.Version) ([]*image.ImageConfig, error) {
// 	var sourceImages []*image.ImageConfig
// 	switch source.Transport() {
// 	case image.DockerTransport:
// 	case image.FileTransport:
// 	}

// 	switch source.Transport() {
// 	case image.DockerTransport:
// 	case image.FileTransport:
// 	}
// 	return nil, nil
// }

// directoryImages generates list to pull from local directory
func directoryImages(registry *image.RegistryConfig) ([]*image.ImageConfig, error) {
	imageDigests := make([]*image.ImageConfig, 0)

	err := filepath.WalkDir(registry.Path(), func(path string, d fs.DirEntry, err error) error {
		// All copied to dir images have this file
		if err != nil || d.IsDir() || d.Name() != "manifest.json" {
			return err
		}

		dirname := filepath.Dir(path)
		// We don't check spllitted directory name, because it may not have digest
		tag, digest, _ := strings.Cut(filepath.Base(dirname), "@")
		additionalPaths := filepath.SplitList(dirname)
		imageDigests = append(imageDigests, image.NewImageConfig(registry, tag, digest, additionalPaths...))
		return filepath.SkipDir
	})

	return imageDigests, err
}

// registryImages generates list of images to pull from deckhouse registry
func registryImages(ctx context.Context, registry *image.RegistryConfig, edition, version, licenseToken string, policyContext *signature.PolicyContext, opts ...image.CopyOption) ([]*image.ImageConfig, error) {
	imagesDigests, err := registryModulesImages()
	if err != nil {
		return nil, err
	}

	auth, err := deckhouseRegistryAuth(edition, licenseToken)
	if err != nil {
		return nil, err
	}
	registryWithAuth := registry.WithAuthConfig(auth)

	newImage := func(tag string, digest string, additionalPaths ...string) *image.ImageConfig {
		additionalPaths = append([]string{edition}, additionalPaths...)
		return image.NewImageConfig(registryWithAuth, tag, digest, additionalPaths...)
	}

	images := make([]*image.ImageConfig, 0, 3+len(imagesDigests)+len(releaseChannels)*3)
	images = append(images, newImage(version, ""), newImage(version, "", "install"), newImage("2", "", "security", "trivy-db"))

	for _, release := range releaseChannels {
		// Check that release-channel image has similar version as used for running dhctl image
		metaImg := newImage(release, "", "release-channel")
		metaVersion, err := fetchReleaseMetadataDeckhouseVersion(ctx, metaImg, policyContext, opts...)
		if err != nil {
			return nil, err
		}
		if metaVersion != version {
			continue
		}

		// Add release images because version of deckhouse is equal to release channel version
		images = append(images, metaImg, newImage(release, ""), newImage(release, "", "install"))
	}

	for tag, digest := range imagesDigests {
		images = append(images, newImage(tag, digest))
	}
	return images, nil
}

// registryModulesImages reads deckhouse module digests file and returns map[<module name> + Titled(<image name>)]<digest>
func registryModulesImages() (map[string]string, error) {
	content, err := os.ReadFile("/deckhouse/candi/images_digests.json")
	if err != nil {
		return nil, err
	}
	var moduleImagesDigests map[string]map[string]string
	if err := json.Unmarshal(content, &moduleImagesDigests); err != nil {
		return nil, err
	}

	imageDigests := make(map[string]string)
	for module, images := range moduleImagesDigests {
		for image, digest := range images {
			fullImageName := module + cases.Title(language.Und, cases.NoLower).String(image)
			imageDigests[fullImageName] = digest
		}
	}
	return imageDigests, nil
}

// fetchReleaseMetadataDeckhouseVersion copies image to local directory and untar it's layers to find version.json and
// returns "version" key found in it
func fetchReleaseMetadataDeckhouseVersion(ctx context.Context, img *image.ImageConfig, policyContext *signature.PolicyContext, opts ...image.CopyOption) (string, error) {
	dir, err := os.MkdirTemp("/tmp", "deckhouse_metadata_*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)

	dirRegisry, err := image.NewRegistry("dir:"+dir, nil)
	if err != nil {
		return "", err
	}

	dest := img.WithNewRegistry(dirRegisry)
	if err := image.CopyImage(ctx, img, dest, policyContext, opts...); err != nil {
		return "", err
	}

	imageDir := filepath.Join(dest.RegistryPath(), img.Tag())
	files, err := os.ReadDir(imageDir)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		contents, err := fileFromGzTarLayer(filepath.Join(imageDir, file.Name()), "version.json")
		if errors.Is(err, io.EOF) || errors.Is(err, tar.ErrHeader) || errors.Is(err, gzip.ErrHeader) {
			continue
		}
		if err != nil {
			return "", err
		}

		var meta struct {
			Version string `json:"version"`
		}

		if err := json.Unmarshal(contents, &meta); err != nil {
			return "", err
		}
		return meta.Version, nil
	}
	return "", fmt.Errorf(`metadata file not found in image from "%s" dir`, dir)
}

// fileFromGzTarLayer finds finds "targetFile" in "archive" tar.gz file
func fileFromGzTarLayer(archive, targetFile string) ([]byte, error) {
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

func copyImage(ctx context.Context, srcImage *image.ImageConfig, destRegistry *image.RegistryConfig, policyContext *signature.PolicyContext, opts ...image.CopyOption) error {
	srcImg := sourceImage(srcImage)
	destImage := destinationImage(destRegistry, srcImage)
	return image.CopyImage(ctx, srcImg, destImage, policyContext, opts...)
}

// sourceImage source destination image
func sourceImage(srcImage *image.ImageConfig) *image.ImageConfig {
	// https://github.com/containers/image/blob/v5.26.1/docker/docker_transport.go#L80
	// If image has both tag and digest we want to pull it with digest
	if srcImage.RegistryTransport() == image.DockerTransport && srcImage.Digest() != "" {
		return srcImage.WithTag("")
	}
	return srcImage
}

// destinationImage prepares destination image
func destinationImage(destRegistry *image.RegistryConfig, srcImage *image.ImageConfig) *image.ImageConfig {
	destImage := srcImage.WithNewRegistry(destRegistry)
	// https://github.com/containers/image/blob/v5.26.1/docker/docker_transport.go#L80
	// If image has both tag and digest we want to push it with tag (because digest will be saved)
	// (because when pushing with digest image becames dangling in the registry)
	if destRegistry.Path() == image.DockerTransport && srcImage.Tag() != "" {
		return destImage.WithDigest("")
	}
	return destImage
}
