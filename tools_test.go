package toolkit

import (
	"fmt"
	"image"
	"image/jpeg"
	"io"

	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
)

func TestTools_RandomString(t *testing.T) {
	var testTools Tools

	s := testTools.RandomString(10)
	if len(s) != 10 {
		t.Errorf("Expected string of length 10, got %d", len(s))
	}
}

var uploadTests = []struct {
	name          string
	allowedTypes  []string
	renameFile    bool
	errorExpected bool
}{
	{
		name:          "allowed no rename",
		allowedTypes:  []string{"image/jpeg", "image/png"},
		renameFile:    false,
		errorExpected: false,
	},
	{
		name:          "allowed rename",
		allowedTypes:  []string{"image/jpeg", "image/png"},
		renameFile:    true,
		errorExpected: false,
	},
	{
		name:          "not allowed",
		allowedTypes:  []string{"image/png"},
		renameFile:    false,
		errorExpected: true,
	},
}

func TestTools_UploadFiles(t *testing.T) {
	for _, e := range uploadTests {
		pr, pw := io.Pipe()
		writer := multipart.NewWriter(pw)
		wg := sync.WaitGroup{}
		wg.Add(1)

		go func() {
			defer writer.Close()
			defer wg.Done()

			part, err := writer.CreateFormFile("file", "./testdata/test.jpeg")
			if err != nil {
				t.Error(err)
			}

			f, err := os.Open("./testdata/test.jpeg")
			if err != nil {
				t.Error(err)
			}
			defer f.Close()

			img, _, err := image.Decode(f)
			if err != nil {
				t.Error(err)
			}

			err = jpeg.Encode(part, img, &jpeg.Options{Quality: 95})
			if err != nil {
				t.Error(err)
			}
		}()

		request := httptest.NewRequest("POST", "/", pr)
		request.Header.Add("Content-Type", writer.FormDataContentType())

		var testTools Tools
		testTools.AllowedFileTypes = e.allowedTypes

		uploadedFiles, err := testTools.UploadFiles(request, "./testdata/uploads", e.renameFile)
		if e.errorExpected && err == nil {
			t.Errorf("%s: Expected error but got none", e.name)
		}

		if !e.errorExpected {
			if _, err := os.Stat(fmt.Sprintf("./testdata/uploads/%s", uploadedFiles[0].NewFileName)); os.IsNotExist(err) {
				t.Errorf("%s: Expected file to be uploaded but it does not exist", e.name)
			}

			// Clean up uploaded file
			os.Remove(fmt.Sprintf("./testdata/uploads/%s", uploadedFiles[0].NewFileName))
		}

		if !e.errorExpected && err != nil {
			t.Errorf("%s: Expected no error but got %v", e.name, err)
		}

		wg.Wait()

	}
}

func TestTools_UploadOneFile(t *testing.T) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer writer.Close()

		part, err := writer.CreateFormFile("file", "./testdata/test.jpeg")
		if err != nil {
			t.Error(err)
		}

		f, err := os.Open("./testdata/test.jpeg")
		if err != nil {
			t.Error(err)
		}
		defer f.Close()

		img, _, err := image.Decode(f)
		if err != nil {
			t.Error(err)
		}

		err = jpeg.Encode(part, img, &jpeg.Options{Quality: 95})
		if err != nil {
			t.Error(err)
		}
	}()

	request := httptest.NewRequest("POST", "/", pr)
	request.Header.Add("Content-Type", writer.FormDataContentType())

	var testTools Tools

	uploadedFile, err := testTools.UploadFile(request, "./testdata/uploads", true)
	if err != nil {
		t.Error(err)
	}

	if _, err := os.Stat(fmt.Sprintf("./testdata/uploads/%s", uploadedFile.NewFileName)); os.IsNotExist(err) {
		t.Errorf("Expected file to be uploaded but it does not exist")
	}

	// Clean up uploaded file
	os.Remove(fmt.Sprintf("./testdata/uploads/%s", uploadedFile.NewFileName))

}

func TestTools_CreateDirIfNotExist(t *testing.T) {
	var testTools Tools

	err := testTools.CreateDirIfNotExist("./testdata/newdir")
	if err != nil {
		t.Error(err)
	}

	err = testTools.CreateDirIfNotExist("./testdata/newdir")
	if err != nil {
		t.Error(err)
	}

	// Clean up created directory
	os.Remove("./testdata/newdir")
}

var slugTests = []struct {
	name          string
	s             string
	expected      string
	errorExpected bool
}{
	{
		name:          "valid string",
		s:             "hello world to the go programming language",
		expected:      "hello-world-to-the-go-programming-language",
		errorExpected: false,
	},
	{
		name:          "empty string",
		s:             "",
		expected:      "",
		errorExpected: true,
	},
	{
		name:          "complex string",
		s:             "hello world! @#$%^&*()",
		expected:      "hello-world",
		errorExpected: false,
	},
	{
		name:          "japanese string",
		s:             "こんにちは世界",
		expected:      "",
		errorExpected: true,
	},
	{
		name:          "japanese string and roman characters",
		s:             "こんにちは世界 hello world",
		expected:      "hello-world",
		errorExpected: false,
	},
}

func TestTools_Slugify(t *testing.T) {
	var testTools Tools

	for _, e := range slugTests {
		slug, err := testTools.Slugify(e.s)
		if e.errorExpected && err == nil {
			t.Errorf("%s: Expected error but got none", e.name)
		}

		if !e.errorExpected && slug != e.expected {
			t.Errorf("%s: Expected slug '%s' but got '%s'", e.name, e.expected, slug)
		}
	}
}

func TestTools_DownloadStaticFile(t *testing.T) {
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)

	var testTools Tools

	testTools.DownloadStaticFile(rr, req, "./testdata", "dog.jpg", "puppy.jpg")

	res := rr.Result()
	defer res.Body.Close()

	if res.Header["Content-Length"][0] != "28981" {
		t.Error("wrong content length of", res.Header["Content-Length"][0])
	}

	if res.Header["Content-Disposition"][0] != "attachment; filename=\"puppy.jpg\"" {
		t.Error("wrong content disposition of", res.Header["Content-Disposition"][0])
	}

	_, err := io.ReadAll(res.Body)
	if err != nil {
		t.Error("error reading response body")
	}
}
