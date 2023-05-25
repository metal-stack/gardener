package registry

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func ExtractFromLayer(image, pathSuffix, dest string) error {
	// TODO(rfranzke): figure this out after breakfast
	image = strings.ReplaceAll(image, "localhost:5001", "garden.local.gardener.cloud:5001")

	imageRef, err := name.ParseReference(image, name.Insecure)
	if err != nil {
		return fmt.Errorf("unable to parse reference: %w", err)
	}

	remoteImage, err := remote.Image(imageRef, remote.WithPlatform(v1.Platform{OS: "linux", Architecture: runtime.GOARCH}))
	if err != nil {
		return fmt.Errorf("unable access remote image reference: %w", err)
	}

	layers, err := remoteImage.Layers()
	if err != nil {
		return fmt.Errorf("unable retrieve image layers: %w", err)
	}

	success := false

	for _, layer := range layers {
		buffer, err := layer.Uncompressed()
		if err != nil {
			return fmt.Errorf("unable to get reader for uncompressed layer: %w", err)
		}

		err = extractTarGz(buffer, pathSuffix, dest)
		if err != nil {
			if errors.Is(err, notFound) {
				continue
			}
			return fmt.Errorf("unable to extract tarball to file system: %w", err)
		}

		success = true
		break
	}

	if !success {
		return fmt.Errorf("did not find file %q in layer", pathSuffix)
	}

	return nil
}

var notFound = errors.New("file not contained in tar")

func extractTarGz(uncompressedStream io.Reader, filenameInArchive, destinationOnOS string) error {
	tarReader := tar.NewReader(uncompressedStream)
	var header *tar.Header
	var err error
	for header, err = tarReader.Next(); err == nil; header, err = tarReader.Next() {
		switch header.Typeflag {
		case tar.TypeReg:
			if !strings.HasSuffix(header.Name, filenameInArchive) {
				continue
			}

			tmpDest := destinationOnOS + ".tmp"

			outFile, err := os.OpenFile(tmpDest, os.O_CREATE|os.O_RDWR, 0755)
			if err != nil {
				return fmt.Errorf("create file failed: %w", err)
			}

			// TODO: handle file close error case
			defer outFile.Close()

			if _, err := io.Copy(outFile, tarReader); err != nil {
				return fmt.Errorf("copying file from tarball failed: %w", err)
			}

			return os.Rename(tmpDest, destinationOnOS)
		default:
			continue
		}
	}
	if err != io.EOF {
		return fmt.Errorf("iterating tar files failed: %w", err)
	}

	return notFound
}
