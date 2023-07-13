package transport

import (
	"context"
	"os"

	"github.com/containers/image/v5/directory"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/types"
)

func init() {
	transports.Register(Transport)
}

// Transport is an ImageTransport for directory paths.
var Transport = tarGzTransport{directory.Transport}

type tarGzTransport struct {
	dir types.ImageTransport
}

// Name returns the name of the transport, which must be unique among other transports.
func (t tarGzTransport) Name() string {
	return "file"
}

// ParseReference converts a string, which should not start with the ImageTransport.Name prefix, into an ImageReference.
func (t tarGzTransport) ParseReference(reference string) (types.ImageReference, error) {
	dirRef, err := t.dir.ParseReference(TrimExt(reference))
	if err != nil {
		return nil, err
	}

	return fileReference{ImageReference: dirRef, archivePath: AddExt(reference)}, nil
}

// ValidatePolicyConfigurationScope checks that scope is a valid name for a signature.PolicyTransportScopes keys
// (i.e. a valid PolicyConfigurationIdentity() or PolicyConfigurationNamespaces() return value).
// It is acceptable to allow an invalid value which will never be matched, it can "only" cause user confusion.
// scope passed to this function will not be "", that value is always allowed.
func (t tarGzTransport) ValidatePolicyConfigurationScope(scope string) error {
	return t.dir.ValidatePolicyConfigurationScope(scope)
}

type fileReference struct {
	types.ImageReference
	archivePath string
}

func (ref fileReference) Transport() types.ImageTransport {
	return Transport
}

// StringWithinTransport returns a string representation of the reference, which MUST be such that
// reference.Transport().ParseReference(reference.StringWithinTransport()) returns an equivalent reference.
// NOTE: The returned string is not promised to be equal to the original input to ParseReference;
// e.g. default attribute values omitted by the user may be filled in in the return value, or vice versa.
// WARNING: Do not use the return value in the UI to describe an image, it does not contain the Transport().Name() prefix.
func (ref fileReference) StringWithinTransport() string {
	return ref.archivePath
}

// NewImage returns a types.ImageCloser for this reference, possibly specialized for this ImageTransport.
// The caller must call .Close() on the returned ImageCloser.
// NOTE: If any kind of signature verification should happen, build an UnparsedImage from the value returned by NewImageSource,
// verify that UnparsedImage, and convert it into a real Image via image.FromUnparsedImage.
// WARNING: This may not do the right thing for a manifest list, see image.FromSource for details.
func (ref fileReference) NewImage(ctx context.Context, sys *types.SystemContext) (types.ImageCloser, error) {
	src, err := newImageSource(ctx, sys, ref)
	if err != nil {
		return nil, err
	}
	return image.FromSource(ctx, sys, src)
}

// NewImageSource returns a types.ImageSource for this reference.
// The caller must call .Close() on the returned ImageSource.
func (ref fileReference) NewImageSource(ctx context.Context, sys *types.SystemContext) (types.ImageSource, error) {
	return newImageSource(ctx, sys, ref)
}

// NewImageDestination returns a types.ImageDestination for this reference.
// The caller must call .Close() on the returned ImageDestination.
func (ref fileReference) NewImageDestination(ctx context.Context, sys *types.SystemContext) (types.ImageDestination, error) {
	return newImageDestination(ctx, sys, ref)
}

func (ref fileReference) close() error {
	return os.RemoveAll(ref.ImageReference.StringWithinTransport())
}
