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
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/containers/image/v5/docker"
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

	deckhouseRegistry = "registry.deckhouse.io/deckhouse"

	destinationHelp = "destination for images to write (directory: 'dir:<directory>' or registry: 'docker://<registry repositroy')."
	fromHelp        = "source directory for downloaded deckhouse images ('dir://<directory>')."
	// dontPullMetadataHelp = "If set, release metadata images (registry.deckhouse.io/deckhouse/(ce|ee)/release-channel:(early-access|alpha|beta|stable|rock-solid)) will not pull."

)

var (
	dockerTransport = docker.Transport.Name()
)

func DefineMirrorCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	var (
		// mirrorRelease          = app.NewStringWithRegexpValidation("(v[0-9]+\\.[0-9]+)\\..+")
		// mirrorEdition          string
		// mirrorDontPullMetadata bool
		licenseToken string

		destination         = app.NewStringWithRegexpValidation("(dir:|docker://).+")
		source              = app.NewStringWithRegexpValidation("(dir:|docker://).+")
		destinationUser     string
		destinationPassword string
		destinationInsecure bool
	)

	cmd := kpApp.Command("mirror", "Copy images from deckhouse registry or fs directory to specified registry or fs directory.")

	cmd.Arg("DESTINATION", destinationHelp).Required().SetValue(&destination)

	cmd.Flag("from", fromHelp).SetValue(&source)
	// Deckhouse registry flags
	// cmd.Flag("release", "Deckhouse release to download, if not set latest release is used.").SetValue(&mirrorRelease)
	// cmd.Flag("edition", "Deckhouse edition to download, possible values ce|ee.").Default(eeEdition).EnumVar(&mirrorEdition, ceEdition, eeEdition)
	// cmd.Flag("do-not-pull-release-metadata-images", dontPullMetadataHelp).BoolVar(&mirrorDontPullMetadata)
	cmd.Flag("license", "License key for Deckhouse registry.").StringVar(&licenseToken)

	// Destination registry flags
	cmd.Flag("username", "Username for the destination registry.").StringVar(&destinationUser)
	cmd.Flag("password", "Password for the destination registry.").StringVar(&destinationPassword)
	cmd.Flag("insecure", "Use http instead of https while connecting to registry.").BoolVar(&destinationInsecure)

	runFunc := func() error {
		ctx := context.Background()

		release, err := deckhouseRelease()
		if err != nil {
			return err
		}

		edition, err := deckhouseEdition()
		if err != nil {
			return err
		}

		images, err := deckhouseImages(source.String(), edition, release, licenseToken)
		if err != nil {
			return err
		}

		policyContext, err := newPolicyContext()
		if err != nil {
			return nil
		}
		defer policyContext.Destroy()

		copyOpts := []image.CopyOptions{
			image.WithCopyAllImages(),
			image.WithPreserveDigests(),
			image.WithOutput(log.GetDefaultLogger()),
		}

		if destinationInsecure {
			copyOpts = append(copyOpts, image.WithDestInsecure())
		}

		// Copy images
		destRegistry := image.NewRegistry(destination.String(), registryAuth(destinationUser, destinationPassword))
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

func newPolicyContext() (*signature.PolicyContext, error) {
	// https://github.com/containers/skopeo/blob/v1.12.0/cmd/skopeo/main.go#L141
	return signature.NewPolicyContext(&signature.Policy{
		Default: signature.PolicyRequirements{signature.NewPRInsecureAcceptAnything()},
	})
}

func deckhouseRelease() (string, error) {
	content, err := os.ReadFile("/deckhouse/release")
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

func deckhouseImages(sourceDir, edition, release, licenseToken string) ([]*image.ImageConfig, error) {
	if sourceDir != "" {
		imagesDigests, err := directoryModulesImages(sourceDir)
		return imagesDigests, err
	}

	d8Auth, err := deckhouseRegistryAuth(edition, licenseToken)
	if err != nil {
		return nil, err
	}

	d8Registry := deckhouseRegistryPath(d8Auth, edition)

	imagesDigests, err := deckhouseModulesImages()
	if err != nil {
		return nil, err
	}

	images := make([]*image.ImageConfig, 0, len(imagesDigests)+4)
	images = append(images, image.NewImageConfig(d8Registry, release, ""))
	images = append(images, image.NewImageConfig(d8Registry, release, "", "install"))
	images = append(images, image.NewImageConfig(d8Registry, release, "", "release-channel"))
	if edition != ceEdition {
		images = append(images, image.NewImageConfig(d8Registry, "2", "", "security", "trivy-db"))
	}

	for tag, digest := range imagesDigests {
		images = append(images, image.NewImageConfig(d8Registry, tag, digest))
	}
	return images, nil
}

func directoryModulesImages(sourceDir string) ([]*image.ImageConfig, error) {
	registry := image.NewRegistry(sourceDir, nil)
	files, err := os.ReadDir(registry.RegistryPath())
	if err != nil {
		return nil, err
	}

	imageDigestsUnique := make(map[string]*image.ImageConfig, 0)
	for _, file := range files {
		if err := directoryImagesRecursion(file, imageDigestsUnique, registry, ""); err != nil {
			return nil, err
		}
	}

	imageDigests := make([]*image.ImageConfig, 0, len(imageDigestsUnique))
	for _, img := range imageDigestsUnique {
		imageDigests = append(imageDigests, img)
	}
	return imageDigests, nil
}

func directoryImagesRecursion(original os.DirEntry, imageDigests map[string]*image.ImageConfig, registry *image.RegistryConfig, parentPaths ...string) error {
	if !original.IsDir() {
		return nil
	}

	newParentPaths := append(parentPaths, original.Name())
	newPath := filepath.Join(newParentPaths...)
	files, err := os.ReadDir(filepath.Join(registry.RegistryPath(), newPath))
	if err != nil {
		return err
	}

	var withDirectories bool
	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		withDirectories = true
		if err := directoryImagesRecursion(file, imageDigests, registry, newParentPaths...); err != nil {
			return err
		}
	}
	if withDirectories {
		return nil
	}

	tag, digest, _ := strings.Cut(original.Name(), "@")
	imageDigests[newPath] = image.NewImageConfig(registry, tag, digest, parentPaths...)
	return nil
}

func deckhouseModulesImages() (map[string]string, error) {
	// srcImage, err := deckhouseImage(ctx, licenseToken)
	// if err != nil {
	// 	return nil, err
	// }

	// deckhouseImage, err := os.MkdirTemp("/tmp", "deckhouse_image_")
	// if err != nil {
	// 	return nil, err
	// }
	// defer os.RemoveAll(deckhouseImage)

	// destTarball := image.NewImageConfig(nil, deckhouseImage)
	// destTarball.SetTransport("dir")

	// opts := []image.CopyOptions{
	// 	image.WithSourceSystemContext(&types.SystemContext{DockerAuthConfig: srcImage.AuthConfig()}),
	// }

	// if err := image.CopyImage(ctx, srcImage, destTarball, policyContext, opts...); err != nil {
	// 	return nil, err
	// }
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

func deckhouseRegistryPath(d8Auth *types.DockerAuthConfig, paths ...string) *image.RegistryConfig {
	paths = append([]string{deckhouseRegistry}, paths...)
	return image.NewRegistry("docker://"+filepath.Join(paths...), d8Auth)
}

func copyImage(ctx context.Context, srcImage *image.ImageConfig, destRegistry *image.RegistryConfig, policyContext *signature.PolicyContext, opts ...image.CopyOptions) error {
	srcImg := sourceImage(srcImage)
	destImage := destinationImage(destRegistry, srcImage)
	return image.CopyImage(ctx, srcImg, destImage, policyContext, opts...)
}

func sourceImage(srcImage *image.ImageConfig) *image.ImageConfig {
	if srcImage.RegistryTransport() == dockerTransport && srcImage.Digest() != "" {
		return srcImage.WithTag("")
	}
	return srcImage
}

func destinationImage(destRegistry *image.RegistryConfig, srcImage *image.ImageConfig) *image.ImageConfig {
	destImage := srcImage.WithNewRegistry(destRegistry)
	if destRegistry.RegistryTransport() == dockerTransport && srcImage.Tag() != "" {
		return destImage.WithDigest("")
	}
	return destImage
}

// func latestDeckhouseRelease(ctx context.Context) (string, error) {
// 	client := github.NewClient(nil)
// 	tags, _, err := client.Repositories.ListTags(ctx, "deckhouse", "deckhouse", &github.ListOptions{PerPage: 1})
// 	if err != nil {
// 		return "", err
// 	}
// 	return tags[0].GetName(), nil
// }

// func digestsFromImageDir(imageDir string) (map[string]map[string]string, error) {
// 	files, err := os.ReadDir(imageDir)
// 	if err != nil {
// 		return nil, err
// 	}

// 	for _, file := range files {
// 		digests, err := untarDigestsFileFromLayer(filepath.Join(imageDir, file.Name()))
// 		if err == nil {
// 			return digests, nil
// 		}
// 	}
// 	return nil, fmt.Errorf("digests file not found in image from '%s' dir", imageDir)
// }

// func untarDigestsFileFromLayer(layerFile string) (map[string]map[string]string, error) {
// 	file, err := os.Open(layerFile)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer file.Close()

// 	buf := bytes.NewBuffer(nil)
// 	tr := tar.NewReader(file)
// 	for {
// 		hdr, err := tr.Next()
// 		if err != nil {
// 			return nil, err
// 		}

// 		if hdr.Name == "deckhouse/modules/images_digests.json" {
// 			break
// 		}
// 	}

// 	if _, err := io.Copy(buf, tr); err != nil {
// 		return nil, err
// 	}

// 	var modulesDigests map[string]map[string]string
// 	return modulesDigests, json.Unmarshal(buf.Bytes(), &modulesDigests)
// }
