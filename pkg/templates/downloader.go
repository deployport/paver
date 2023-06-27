package templates

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DecompressFromURL downloads a tar.gz file from a URL and decompresses it
func DecompressFromURL(url string) (string, error) {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Create a temporary directory
	dir, err := ioutil.TempDir("", "tar.gz")
	if err != nil {
		return "", err
	}

	// Open the tar archive for reading.
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	// Iterate through the files in the archive.
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return "", err
		}
		target := filepath.Join(dir, hdr.Name)

		// Check the type of the file
		switch hdr.Typeflag {
		case tar.TypeDir: // if a directory
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return "", err
				}
			}
		case tar.TypeReg: // if a file
			dir := filepath.Dir(target)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return "", err
				}
			}

			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(hdr.Mode))
			if err != nil {
				return "", err
			}

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return "", err
			}
			f.Close()
		case tar.TypeSymlink, tar.TypeLink:
			link := filepath.Join(dir, hdr.Linkname)
			if err := os.Symlink(link, target); err != nil {
				return "", err
			}
		}

		// Handle any trailing slashes by trimming them
		target = strings.TrimSuffix(target, "/")
	}
	return dir, nil
}

// GetTempTemplateSubDirectory returns the subdirectory of a template directory
func GetTempTemplateSubDirectory(dir string) (string, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return "", err
	}

	subdirs := make([]string, 0)
	for _, file := range files {
		if file.IsDir() {
			subdirs = append(subdirs, file.Name())
		}
	}

	if len(subdirs) != 1 {
		return dir, nil
	}

	return filepath.Join(dir, subdirs[0]), nil
}
