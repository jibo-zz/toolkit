package toolkit

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const randomStringSource = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_+"

// Tools is the type used to instantiate the module. Any variable of this type will have access
// to all the methods with a receiver *Tools.
type Tools struct {
	MaxFileSize      int64
	AllowedFileTypes []string
}

// RandomString returns a string of random characters of length n. Useing randomStringSource as the source of characters to choose from.
// The random string is generated using crypto/rand for better randomness.
func (t *Tools) RandomString(n int) string {
	s, r := make([]rune, n), []rune(randomStringSource)
	for i := range s {
		p, _ := rand.Prime(rand.Reader, len(r))
		x, y := p.Uint64(), uint64(len(r))
		s[i] = r[x%y]
	}
	return string(s)
}

// UploadedFile represents an uploaded file with its metadata.
type UploadedFile struct {
	NewFileName      string
	OriginalFileName string
	FileSize         int64
}

func (t *Tools) UploadFile(r *http.Request, uploadDir string, rename ...bool) (*UploadedFile, error) {
	renameFile := true
	if len(rename) > 0 {
		renameFile = rename[0]
	}

	files, err := t.UploadFiles(r, uploadDir, renameFile)
	if err != nil {
		return nil, err
	}

	return files[0], nil
}

func (t *Tools) UploadFiles(r *http.Request, uploadDir string, rename ...bool) ([]*UploadedFile, error) {
	renameFile := true
	if len(rename) > 0 {
		renameFile = rename[0]
	}

	var uploadedFiles []*UploadedFile

	if t.MaxFileSize == 0 {
		t.MaxFileSize = 1024 * 1024 * 1024
	}

	err := t.CreateDirIfNotExist(uploadDir)
	if err != nil {
		return nil, err
	}

	err = r.ParseMultipartForm(t.MaxFileSize)
	if err != nil {
		return nil, errors.New("The uploaded file is too big")
	}

	for _, fheaders := range r.MultipartForm.File {
		for _, hdr := range fheaders {
			uploadedFiles, err = func(uploadedFiles []*UploadedFile) ([]*UploadedFile, error) {
				var uploadedFile UploadedFile
				infile, err := hdr.Open()
				if err != nil {
					return nil, err
				}
				defer infile.Close()

				buff := make([]byte, 512)
				_, err = infile.Read(buff)
				if err != nil {
					return nil, err
				}

				// check to see if the file type is permitted
				allowed := false
				filetype := http.DetectContentType(buff)

				if len(t.AllowedFileTypes) > 0 {
					for _, ft := range t.AllowedFileTypes {
						if strings.EqualFold(filetype, ft) {
							allowed = true
						}
					}
				} else {
					allowed = true
				}

				if !allowed {
					return nil, errors.New("The uploaded file type is not allowed")
				}

				_, err = infile.Seek(0, 0)
				if err != nil {
					return nil, err
				}

				if renameFile {
					uploadedFile.NewFileName = fmt.Sprintf("%s%s", t.RandomString(25), filepath.Ext(hdr.Filename))
				} else {
					uploadedFile.NewFileName = hdr.Filename
				}

				uploadedFile.OriginalFileName = hdr.Filename

				var outfile *os.File
				defer outfile.Close()

				if outfile, err = os.Create(filepath.Join(uploadDir, uploadedFile.NewFileName)); err != nil {
					return nil, err
				} else {
					fileSize, err := io.Copy(outfile, infile)
					if err != nil {
						return nil, err
					}
					uploadedFile.FileSize = fileSize
				}

				uploadedFiles = append(uploadedFiles, &uploadedFile)

				return uploadedFiles, nil
			}(uploadedFiles)
			if err != nil {
				return uploadedFiles, err
			}
		}
	}
	return uploadedFiles, nil
}

func (t *Tools) CreateDirIfNotExist(dir string) error {
	const mode = 0755
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, mode)
		if err != nil {
			return err
		}
	}
	return nil
}

// Slugify takes a string and converts it to a slug format. It removes all non-alphanumeric characters, replaces spaces with hyphens, and converts the string to lowercase.
func (t *Tools) Slugify(s string) (string, error) {
	if s == "" {
		return "", errors.New("input string cannot be empty")
	}

	var re = regexp.MustCompile(`[^a-z\d]+`)
	slug := strings.Trim(re.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if len(slug) == 0 {
		return "", errors.New("slug cannot be empty after processing")
	}
	return slug, nil
}

// DownloadStaticFile downloads a file, and tries to force the browser to avoid displaying it
// in the browser window by setting the Content-Disposition. It also allows specification of
// the display name.
func (t *Tools) DownloadStaticFile(w http.ResponseWriter, r *http.Request, path, file, displayName string) {
	fullPath := filepath.Join(path, file)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", displayName))

	http.ServeFile(w, r, fullPath)
}
