package registry

import (
	"context"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	containerv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

type ImageGetter interface {
	GetImageVersion(ctx context.Context, keychain authn.Keychain, transport *http.Transport, imageName string) (ImageVersion, error)
	PullImageInfo(ctx context.Context, keychain authn.Keychain, transport *http.Transport, imageName string) (*containerv1.Image, error)
}

type ImageVersion struct {
	Version string
	Digest  digest.Digest
}

type Client struct{}

const (
	// VersionLabel is the name of the label used on ActiveGate-provided images.
	VersionLabel = "com.dynatrace.build-version"
)

func NewClient() *Client {
	return &Client{}
}

func (c *Client) GetImageVersion(ctx context.Context, keychain authn.Keychain, transport *http.Transport, imageName string) (ImageVersion, error) {
	ref, err := name.ParseReference(imageName)
	if err != nil {
		return ImageVersion{}, errors.WithMessagef(err, "parsing reference %q", imageName)
	}

	options := []remote.Option{
		remote.WithContext(ctx),
		remote.WithTransport(transport),
	}
	if keychain != nil {
		options = append(options, remote.WithAuthFromKeychain(keychain))
	}

	descriptor, err := remote.Get(ref, options...)
	if err != nil {
		return ImageVersion{}, errors.WithMessagef(err, "getting reference %q", ref)
	}

	// TODO: does not work for indexes which contain schema v1 manifests
	img, err := descriptor.Image()
	if err != nil {
		return ImageVersion{}, errors.WithMessagef(err, "descriptor.Image()")
	}

	// use image digest as a fallback
	digestFn := img.Digest

	// try to get image manifest to cover multi arch images
	imageIndex, err := descriptor.ImageIndex()
	if err == nil {
		digestFn = imageIndex.Digest
	}

	dig, err := digestFn()
	if err != nil {
		return ImageVersion{}, errors.WithMessagef(err, "could not get image digest")
	}

	cf, err := img.ConfigFile()
	if err != nil {
		return ImageVersion{}, errors.WithMessagef(err, "img.ConfigFile")
	}

	return ImageVersion{
		Digest:  digest.Digest(dig.String()),
		Version: cf.Config.Labels[VersionLabel], // empty if unset
	}, nil
}

func (c *Client) PullImageInfo(ctx context.Context, keychain authn.Keychain, transport *http.Transport, imageName string) (*containerv1.Image, error) {
	ref, err := name.ParseReference(imageName)
	if err != nil {
		return nil, errors.WithMessagef(err, "parsing reference %q:", imageName)
	}

	image, err := remote.Image(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(keychain), remote.WithTransport(transport))
	if err != nil {
		return nil, errors.WithMessagef(err, "getting image %q", imageName)
	}

	return &image, nil
}