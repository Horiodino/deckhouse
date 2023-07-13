package transport

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	tarGzExt = ".tar.gz"
)

func TrimExt(s string) string {
	return strings.TrimSuffix(s, tarGzExt)
}

func AddExt(s string) string {
	return TrimExt(s) + tarGzExt
}

func ExtractTarGz(filename string) error {
	resultPath := func(s string) string { return filepath.Join(TrimExt(filename), s) }

	gzipStream, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer gzipStream.Close()

	uncompressedStream, err := gzip.NewReader(gzipStream)
	if err != nil {
		return err
	}
	defer uncompressedStream.Close()

	tarReader := tar.NewReader(uncompressedStream)

	var header *tar.Header
	for header, err = tarReader.Next(); err == nil; header, err = tarReader.Next() {
		switch header.Typeflag {
		case tar.TypeDir:
			err = os.Mkdir(resultPath(header.Name), header.FileInfo().Mode().Perm())
		case tar.TypeReg:
			err = mkFile(resultPath(header.Name), tarReader, header.FileInfo())
		case tar.TypeSymlink:
			err = os.Symlink(resultPath(header.Name), header.Linkname)
		case tar.TypeLink:
			err = os.Link(resultPath(header.Name), header.Linkname)
		default:
			err = fmt.Errorf("extractTarGz: uknown type: %b in %s", header.Typeflag, header.Name)
		}
		if err != nil {
			return err
		}
	}
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func compressDir(dirname string) error {
	file, err := os.Create(AddExt(dirname))
	if err != nil {
		return err
	}
	defer file.Close()

	writer, err := gzip.NewWriterLevel(file, gzip.BestCompression)
	if err != nil {
		return err
	}
	defer writer.Close()

	tw := tar.NewWriter(writer)
	defer tw.Close()

	walkFn := func(path string, info os.FileInfo, err error) error {
		if info.Mode().IsDir() || err != nil {
			return err
		}
		// Because of scoping we can reference the external root_directory variable
		newPath := path[len(dirname):]
		if len(newPath) == 0 {
			return nil
		}
		fr, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fr.Close()

		h, err := tar.FileInfoHeader(info, newPath)
		if err != nil {
			return err
		}

		h.Name = newPath
		if err = tw.WriteHeader(h); err != nil {
			return err
		}

		_, err = io.Copy(tw, fr)
		return err
	}

	return filepath.Walk(dirname, walkFn)
}

func mkFile(name string, content io.Reader, info os.FileInfo) error {
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, content)
	return err
}
