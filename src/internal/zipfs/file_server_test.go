package zipfs

import (
	"bytes"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestResponseWriter struct {
	header http.Header
	status int
	buf    bytes.Buffer
}

func NewTestResponseWriter() *TestResponseWriter {
	return &TestResponseWriter{
		header: make(http.Header),
		status: 200,
	}
}

func (w *TestResponseWriter) Header() http.Header {
	return w.header
}

func (w *TestResponseWriter) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}

func (w *TestResponseWriter) WriteHeader(status int) {
	w.status = status
}

func TestNew(t *testing.T) {
	assertion := assert.New(t)
	testCases := []struct {
		Name  string
		Error string
	}{
		{
			Name:  "testdata/does-not-exist.zip",
			Error: "The system cannot find the file specified",
		},
		{
			Name:  "testdata/testdata.zip",
			Error: "",
		},
		{
			Name:  "testdata/not-a-zip-file.txt",
			Error: "zip: not a valid zip file",
		},
	}

	for _, tc := range testCases {
		fs, err := New(tc.Name)
		if tc.Error != "" {
			assertion.Error(err)
			//assertion.True(strings.Contains(err.Error(), tc.Error), err.Error())
			assertion.Nil(fs)
		} else {
			assertion.NoError(err)
			assertion.NotNil(fs)
		}
		if fs != nil {
			_ = fs.Close()
		}
	}
}

func TestServeHTTP(t *testing.T) {
	assertion := assert.New(t)
	required := require.New(t)

	fs, err := New("testdata/testdata.zip")
	required.NoError(err)
	required.NotNil(fs)

	handler := FileServer(fs)

	testCases := []struct {
		Path            string
		Headers         []string
		Status          int
		ContentType     string
		ContentLength   string
		ContentEncoding string
		ETag            string
		Size            int
		Location        string
	}{
		{
			Path:   "/img/circle.png",
			Status: 200,
			Headers: []string{
				"Accept-Encoding: deflate, gzip",
			},
			ContentType:     "image/png",
			ContentLength:   "4758",
			ContentEncoding: "deflate",
			Size:            4758,
			ETag:            `"1755529fb2ff"`,
		},
		{
			Path:   "/img/circle.png",
			Status: 200,
			Headers: []string{
				"Accept-Encoding: gzip",
			},
			ContentType:     "image/png",
			ContentLength:   "5973",
			ContentEncoding: "",
			Size:            5973,
			ETag:            `"1755529fb2ff"`,
		},
		{
			Path:   "/",
			Status: 200,
			Headers: []string{
				"Accept-Encoding: deflate, gzip",
			},
			ContentType:     "text/html; charset=utf-8",
			ContentEncoding: "deflate",
		},
		{
			Path:            "/test.html",
			Status:          200,
			Headers:         []string{},
			ContentType:     "text/html; charset=utf-8",
			ContentEncoding: "",
		},
		{
			Path:   "/does/not/exist",
			Status: 404,
			Headers: []string{
				"Accept-Encoding: deflate, gzip",
			},
			ContentType: "text/plain; charset=utf-8",
		},
		{
			Path:   "/random.dat",
			Status: 200,
			Headers: []string{
				"Accept-Encoding: deflate",
			},
			ContentType:     getMimeType(".dat"),
			ContentLength:   "10000",
			ContentEncoding: "",
			Size:            10000,
			ETag:            `"27106c15f45b"`,
		},
		{
			Path:            "/random.dat",
			Status:          200,
			Headers:         []string{},
			ContentType:     getMimeType(".dat"),
			ContentLength:   "10000",
			ContentEncoding: "",
			Size:            10000,
			ETag:            `"27106c15f45b"`,
		},
		{
			Path:   "/random.dat",
			Status: 206,
			Headers: []string{
				`If-Range: "27106c15f45b"`,
				"Range: bytes=0-499",
			},
			ContentType:     getMimeType(".dat"),
			ContentLength:   "500",
			ContentEncoding: "",
			Size:            500,
			ETag:            `"27106c15f45b"`,
		},
		{
			Path:   "/random.dat",
			Status: 200,
			Headers: []string{
				`If-Range: "123456789"`,
				"Range: bytes=0-499",
				"Accept-Encoding: deflate, gzip",
			},
			ContentType:     getMimeType(".dat"),
			ContentLength:   "10000",
			ContentEncoding: "",
			Size:            10000,
			ETag:            `"27106c15f45b"`,
		},
		{
			Path:   "/random.dat",
			Status: 304,
			Headers: []string{
				`If-None-Match: "27106c15f45b"`,
				"Accept-Encoding: deflate, gzip",
			},
			ContentType:     "",
			ContentLength:   "",
			ContentEncoding: "",
			Size:            0,
			ETag:            `"27106c15f45b"`,
		},
		{
			Path:   "/random.dat",
			Status: 304,
			Headers: []string{
				fmt.Sprintf("If-Modified-Since: %s", time.Now().UTC().Add(time.Hour * 10000).Format(http.TimeFormat)),
				"Accept-Encoding: deflate, gzip",
			},
			ContentType:     "",
			ContentLength:   "",
			ContentEncoding: "",
			Size:            0,
		},
		{
			Path:          "random.dat",
			Status:        200,
			Headers:       []string{},
			ContentType:   getMimeType(".dat"),
			ContentLength: "10000",
			Size:          10000,
			ETag:          `"27106c15f45b"`,
		},
		{
			Path:     "/index.html",
			Status:   301,
			Headers:  []string{},
			Location: "./",
		},
		{
			Path:     "/empty",
			Status:   301,
			Headers:  []string{},
			Location: "empty/",
		},
		{
			Path:     "/img/circle.png/",
			Status:   301,
			Headers:  []string{},
			Location: "../circle.png",
		},
	}

	for _, tc := range testCases {
		req := &http.Request{
			URL: &url.URL{
				Scheme: "http",
				Host:   "test-server.com",
				Path:   tc.Path,
			},
			Header: make(http.Header),
			Method: "GET",
		}

		for _, header := range tc.Headers {
			arr := strings.SplitN(header, ":", 2)
			key := strings.TrimSpace(arr[0])
			value := strings.TrimSpace(arr[1])
			req.Header.Add(key, value)
		}

		w := NewTestResponseWriter()
		handler.ServeHTTP(w, req)

		assertion.Equal(tc.Status, w.status, tc.Path)
		assertion.Equal(tc.ContentType, w.Header().Get("Content-Type"), tc.Path)
		if tc.ContentLength != "" {
			// only check content length for non-text because length will differ
			// between windows and unix
			assertion.Equal(tc.ContentLength, w.Header().Get("Content-Length"), tc.Path)
		}
		assertion.Equal(tc.ContentEncoding, w.Header().Get("Content-Encoding"), tc.Path)
		if tc.Size > 0 {
			assertion.Equal(tc.Size, w.buf.Len(), tc.Path)
		}
		if tc.ETag != "" {
			// only check ETag for non-text files because CRC will differ between
			// windows and unix
			assertion.Equal(tc.ETag, w.Header().Get("Etag"), tc.Path)
		}
		if tc.Location != "" {
			assertion.Equal(tc.Location, w.Header().Get("Location"), tc.Path)
		}
	}
}

func TestToHTTPError(t *testing.T) {
	assertion := assert.New(t)

	testCases := []struct {
		Err     error
		Message string
		Status  int
	}{
		{
			Err:     os.ErrNotExist,
			Message: "404 page not found",
			Status:  404,
		},
		{
			Err:     errors.New("test error condition"),
			Message: "500 Internal Server Error",
			Status:  500,
		},
	}

	for _, tc := range testCases {
		msg, code := toHTTPError(tc.Err)
		assertion.Equal(tc.Message, msg, tc.Err.Error())
		assertion.Equal(tc.Status, code, tc.Err.Error())
		msg, code = toHTTPError(&os.PathError{Op: "op", Path: "path", Err: tc.Err})
		assertion.Equal(tc.Message, msg, tc.Err.Error())
		assertion.Equal(tc.Status, code, tc.Err.Error())
	}
}

func TestLocalRedirect(t *testing.T) {
	assertion := assert.New(t)

	testCases := []struct {
		Url      string
		NewPath  string
		Location string
	}{
		{
			Url:      "/test",
			NewPath:  "./test/",
			Location: "./test/",
		},
		{
			Url:      "/test?a=32&b=54",
			NewPath:  "./test/",
			Location: "./test/?a=32&b=54",
		},
	}

	for _, tc := range testCases {
		u, err := url.Parse(tc.Url)
		assertion.NoError(err)
		r := &http.Request{
			URL: u,
		}
		w := NewTestResponseWriter()
		localRedirect(w, r, tc.NewPath)
		assertion.Equal(http.StatusMovedPermanently, w.status)
		assertion.Equal(tc.Location, w.Header().Get("Location"))
	}
}

func TestCheckETag(t *testing.T) {
	assertion := assert.New(t)

	testCases := []struct {
		ModTime       time.Time
		Method        string
		Etag          string
		Range         string
		IfRange       string
		IfNoneMatch   string
		ContentType   string
		ContentLength string

		RangeReq string
		Done     bool
	}{
		{
			// Using the modified time instead of the ETag in If-Range header
			// If-None-Match is not set.
			ModTime:       time.Date(2006, 4, 12, 15, 4, 5, 0, time.UTC),
			Method:        "GET",
			Etag:          `"xxxxyyyy"`,
			Range:         "bytes=500-999",
			IfRange:       `Wed, 12 Apr 2006 15:04:05 GMT`,
			ContentType:   "text/html",
			ContentLength: "2024",

			RangeReq: "bytes=500-999",
			Done:     false,
		},
		{
			// Using the modified time instead of the ETag in If-Range header
			// If-None-Match is set.
			ModTime:       time.Date(2006, 4, 12, 15, 4, 5, 0, time.UTC),
			Method:        "GET",
			Etag:          `"xxxxyyyy"`,
			Range:         "bytes=500-999",
			IfRange:       `Wed, 12 Apr 2006 15:04:05 GMT`,
			IfNoneMatch:   `"xxxxyyyy"`,
			ContentType:   "text/html",
			ContentLength: "2024",

			RangeReq: "",
			Done:     true,
		},
		{
			// ETag not set, but If-None-Match is.
			ModTime:       time.Date(2006, 4, 12, 15, 4, 5, 0, time.UTC),
			Method:        "GET",
			IfNoneMatch:   `"xxxxyyyy"`,
			ContentType:   "text/html",
			ContentLength: "2024",

			RangeReq: "",
			Done:     false,
		},
		{
			// ETag matches If-None-Match, but method is not GET or HEAD
			ModTime:       time.Date(2006, 4, 12, 15, 4, 5, 0, time.UTC),
			Method:        "POST",
			Etag:          `"xxxxyyyy"`,
			IfNoneMatch:   `"xxxxyyyy"`,
			ContentType:   "text/html",
			ContentLength: "2024",

			RangeReq: "",
			Done:     false,
		},
		{
			// Using the ETag in the If-Range header
			ModTime:       time.Date(2006, 4, 12, 15, 4, 5, 0, time.UTC),
			Method:        "GET",
			Etag:          `"xxxxyyyy"`,
			Range:         "bytes=500-999",
			IfRange:       `"xxxxyyyy"`,
			ContentType:   "text/html",
			ContentLength: "2024",

			RangeReq: "bytes=500-999",
			Done:     false,
		},
		{
			// Using an out of date ETag in the If-Range header
			ModTime:       time.Date(2006, 4, 12, 15, 4, 5, 0, time.UTC),
			Method:        "GET",
			Etag:          `"xxxxyyyy"`,
			Range:         "bytes=500-999",
			IfRange:       `"aaaabbbb"`,
			ContentType:   "text/html",
			ContentLength: "2024",

			RangeReq: "",
			Done:     false,
		},
		{
			// Using an out of date ETag in the If-Range header
			ModTime:       time.Date(2006, 4, 12, 15, 4, 5, 0, time.UTC),
			Method:        "GET",
			Etag:          `"xxxxyyyy"`,
			Range:         "bytes=500-999",
			IfRange:       `"aaaabbbb"`,
			ContentType:   "text/html",
			ContentLength: "2024",

			RangeReq: "",
			Done:     false,
		},
	}

	for i, tc := range testCases {
		r := &http.Request{Method: tc.Method, Header: http.Header{}}
		w := NewTestResponseWriter()
		if tc.Etag != "" {
			w.Header().Add("Etag", tc.Etag)
		}
		if tc.Range != "" {
			r.Header.Add("Range", tc.Range)
		}
		if tc.IfRange != "" {
			r.Header.Add("If-Range", tc.IfRange)
		}
		if tc.IfNoneMatch != "" {
			r.Header.Add("If-None-Match", tc.IfNoneMatch)
		}
		if tc.ContentType != "" {
			w.Header().Add("Content-Type", tc.ContentType)
		}
		if tc.ContentLength != "" {
			w.Header().Add("Content-Length", tc.ContentLength)
		}
		_ = "breakpoint"
		rangeReq, done := checkETag(w, r, tc.ModTime)
		assertion.Equal(tc.RangeReq, rangeReq, fmt.Sprintf("test case #%d", i))
		assertion.Equal(tc.Done, done, fmt.Sprintf("test case #%d", i))
		if done {
			assertion.Equal("", w.Header().Get("Content-Length"))
			assertion.Equal("", w.Header().Get("Content-Type"))
		} else {
			assertion.Equal(tc.ContentLength, w.Header().Get("Content-Length"))
			assertion.Equal(tc.ContentType, w.Header().Get("Content-Type"))
		}
	}
}

func TestCheckLastModified(t *testing.T) {
	assertion := assert.New(t)

	testCases := []struct {
		ModTime         time.Time
		IfModifiedSince string
		ContentType     string
		ContentLength   string
		LastModified    string
		Status          int
		Done            bool
	}{
		{
			ModTime:         time.Date(2020, 8, 1, 15, 3, 41, 0, time.UTC),
			IfModifiedSince: "Sat, 01 Aug 2020 15:03:41 GMT",
			ContentType:     "text/html",
			ContentLength:   "3000",
			Status:          http.StatusNotModified,
			Done:            true,
		},
		{
			ModTime:         time.Date(2020, 8, 1, 15, 3, 41, 0, time.UTC),
			IfModifiedSince: "Sat, 01 Aug 2020 15:03:40 GMT",
			ContentType:     "text/html",
			ContentLength:   "3000",
			LastModified:    "Sat, 01 Aug 2020 15:03:41 GMT",
			Status:          http.StatusOK,
			Done:            false,
		},
		{
			ModTime:         time.Time{},
			IfModifiedSince: "Sat, 01 Aug 2020 15:03:40 GMT",
			ContentType:     "text/html",
			ContentLength:   "3000",
			Status:          http.StatusOK,
			Done:            false,
		},
		{
			ModTime:         time.Unix(0, 0),
			IfModifiedSince: "Sat, 01 Aug 2020 15:03:40 GMT",
			ContentType:     "text/html",
			ContentLength:   "3000",
			Status:          http.StatusOK,
			Done:            false,
		},
	}

	for i, tc := range testCases {
		r := &http.Request{Header: http.Header{}}
		w := NewTestResponseWriter()
		if tc.IfModifiedSince != "" {
			r.Header.Set("If-Modified-Since", tc.IfModifiedSince)
		}
		if tc.ContentType != "" {
			w.Header().Set("Content-Type", tc.ContentType)
		}
		if tc.ContentLength != "" {
			w.Header().Set("Content-Length", tc.ContentLength)
		}
		done := checkLastModified(w, r, tc.ModTime)
		failText := fmt.Sprintf("test case #%d", i)
		assertion.Equal(tc.Done, done, failText)
		assertion.Equal(tc.Status, w.status, failText)
		if tc.LastModified != "" {
			assertion.Equal(tc.LastModified, w.Header().Get("Last-Modified"), failText)
		}
		if done {
			assertion.Equal("", w.Header().Get("Content-Type"))
			assertion.Equal("", w.Header().Get("Content-Length"))
		} else {
			assertion.Equal(tc.ContentType, w.Header().Get("Content-Type"))
			assertion.Equal(tc.ContentLength, w.Header().Get("Content-Length"))
		}
	}
}

func getMimeType(ext string) string {
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return mimeType
}
