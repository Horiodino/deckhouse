package main

import (
	"archive/tar"
	"io"
	"log"
	"os"
)

func main() {
	f, err := prepareFileForTarAppend("test.tar")
	if err != nil {
		log.Println(err)
		return
	}
	defer f.Close()
	// Open up the file and append more things to it

	if _, err = f.Seek(-2<<9, io.SeekEnd); err != nil {
		log.Println(err)
		return
	}

	tw := tar.NewWriter(f)
	defer tw.Close()

	test, err := os.ReadFile("/Users/alexmakh/golang/src/github.com/alex123012/isoMiRmap/MappingBundles/miRBase/IsoMiRmapTable_v1.LookupTable.isomiRs.miRBase_v22.txt.gz")
	if err != nil {
		log.Println(err)
		return
	}

	hdr := &tar.Header{
		Name: "foo.bar",
		Size: int64(len(test)),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		log.Println(err)
		return
	}

	if _, err := tw.Write([]byte(test)); err != nil {
		log.Println(err)
		return
	}

	if err := tw.Close(); err != nil {
		log.Println(err)
		return
	}

}

func prepareFileForTarAppend(filename string) (*os.File, error) {
	_, err := os.Stat(filename)
	switch {
	case os.IsNotExist(err):
		f, err := os.Create(filename)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		if err := tar.NewWriter(f).Close(); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	}
	return os.OpenFile(filename, os.O_RDWR, os.ModePerm)
}
