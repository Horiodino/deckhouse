package transport

import (
	"context"

	"github.com/containers/image/v5/types"
)

type fileImageSource struct {
	ref fileReference
	types.ImageSource
}

// newImageSource returns an ImageSource reading from an existing directory.
// The caller must call .Close() on the returned ImageSource.
func newImageSource(ctx context.Context, sys *types.SystemContext, ref fileReference) (types.ImageSource, error) {
	dirSrc, err := ref.ImageReference.NewImageSource(ctx, sys)
	if err != nil {
		return nil, err
	}

	return &fileImageSource{ref: ref, ImageSource: dirSrc}, ExtractTarGz(ref.archivePath)
}

// Reference returns the reference used to set up this source, _as specified by the user_
// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
func (s *fileImageSource) Reference() types.ImageReference {
	return s.ref
}

// Close removes resources associated with an initialized ImageSource, if any.
func (s *fileImageSource) Close() error {
	if err := s.ref.close(); err != nil {
		return err
	}
	return s.ImageSource.Close()
}
