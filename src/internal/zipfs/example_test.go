package zipfs_test

import (
	"net/http"
	"zipsv/internal/zipfs"
)

func Example() {
	fs, err := zipfs.New("testdata/testdata.zip")
	if err != nil {
		return
	}

	_ = http.ListenAndServe(":8080", zipfs.FileServer(fs))
}
