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
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/image"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	eeEdition = "ee"
	ceEdition = "ce"

	destinationHelp = "destination for images to write (directory: 'dir:<directory>' or registry: 'docker://<registry repositroy')."
	sourceHelp      = "source for deckhouse images ('dir://<directory>')."
)

var (
	releaseChannels = [5]string{"alpha", "beta", "early-access", "stable", "rock-solid"}
)

func DefineMirrorCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	var (
		licenseToken string

		destination         = app.NewStringWithRegexpValidation("(dir:|docker://).+")
		source              = app.NewStringWithRegexpValidation("(dir:|docker://).+")
		destinationUser     string
		destinationPassword string
		destinationInsecure bool
		dryRun              bool
	)

	cmd := kpApp.Command("mirror", "Copy images from deckhouse registry or fs directory to specified registry or fs directory.")

	cmd.Arg("DESTINATION", destinationHelp).Required().SetValue(destination)
	cmd.Flag("from", sourceHelp).Default("docker://registry.deckhouse.io/deckhouse").SetValue(source)

	cmd.Flag("dry-run", "Run without actually copying data.").BoolVar(&dryRun)

	// Deckhouse registry flags
	cmd.Flag("license", "License key for Deckhouse registry.").StringVar(&licenseToken)

	// Destination registry flags
	cmd.Flag("username", "Username for the destination registry.").StringVar(&destinationUser)
	cmd.Flag("password", "Password for the destination registry.").StringVar(&destinationPassword)
	cmd.Flag("insecure", "Use http instead of https while connecting to registry.").BoolVar(&destinationInsecure)

	runFunc := func() error {
		ctx := context.Background()

		version, err := deckhouseVersion()
		if err != nil {
			return err
		}

		edition, err := deckhouseEdition()
		if err != nil {
			return err
		}

		policyContext, err := image.NewPolicyContext()
		if err != nil {
			return nil
		}
		defer policyContext.Destroy()

		copyOpts := []image.CopyOption{
			image.WithCopyAllImages(),
			image.WithPreserveDigests(),
			image.WithOutput(log.GetDefaultLogger()),
		}

		if destinationInsecure {
			copyOpts = append(copyOpts, image.WithDestInsecure())
		}

		images, err := deckhouseImages(ctx, source.String(), edition, version, licenseToken, policyContext, copyOpts...)
		if err != nil {
			return err
		}

		destRegistry, err := image.NewRegistry(destination.String(), registryAuth(destinationUser, destinationPassword))
		if err != nil {
			return err
		}

		if dryRun {
			copyOpts = append(copyOpts, image.WithDryRun())
		}

		// Copy images
		for _, srcImage := range images {
			if err := copyImage(ctx, srcImage, destRegistry, policyContext, copyOpts...); err != nil {
				return err
			}
		}
		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		return log.Process("mirror", "Copy images", runFunc)
	})
	return cmd
}

func deckhouseVersion() (string, error) {
	content, err := os.ReadFile("/deckhouse/version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func deckhouseEdition() (string, error) {
	content, err := os.ReadFile("/deckhouse/edition")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func deckhouseImages(ctx context.Context, source, edition, version, licenseToken string, policyContext *signature.PolicyContext, opts ...image.CopyOption) ([]*image.ImageConfig, error) {
	registry, err := image.NewRegistry(source, nil)
	if err != nil {
		return nil, err
	}
	switch registry.RegistryTransport() {
	case image.DockerTransport:
		return registryImages(ctx, registry, edition, version, licenseToken, policyContext, opts...)
	case image.DirTransport:
		return directoryImages(registry)
	}
	return nil, fmt.Errorf("no such transport for source: %s", registry.RegistryTransport())
}

// directoryImages generates list to pull from local directory
func directoryImages(registry *image.RegistryConfig) ([]*image.ImageConfig, error) {
	imageDigests := make([]*image.ImageConfig, 0)

	err := filepath.WalkDir(registry.RegistryPath(), func(path string, d fs.DirEntry, err error) error {
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
	images = append(images, newImage(version, ""), newImage(version, "", "install"))

	if edition != ceEdition {
		images = append(images, newImage("2", "", "security", "trivy-db"))
	}

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

func deckhouseRegistryAuth(edition, licenseToken string) (*types.DockerAuthConfig, error) {
	if edition != ceEdition && licenseToken == "" {
		return nil, errors.New("license is required to download Deckhouse Enterprise Edition. Please provide it with CLI argument --license")
	}

	if edition == ceEdition {
		return nil, nil
	}

	return registryAuth("license-token", licenseToken), nil
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
	return "", fmt.Errorf("metadata file not found in image from '%s' dir", dir)
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
	if destRegistry.RegistryTransport() == image.DockerTransport && srcImage.Tag() != "" {
		return destImage.WithDigest("")
	}
	return destImage
}
