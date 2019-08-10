package zipsv

import (
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"

	"dwap/internal/zipfs"

	"github.com/go-chi/chi"
	"github.com/pseidemann/finish"
)

type Config struct {
	// server port
	Port string

	// server host, to allow local network connection, use "0.0.0.0".
	Host string

	// zip file location
	ZipFile string

	// the key name to be added on the URL, for example;
	// "localhost:8080/a.js?key=123456", here, the key name is "key",
	// and, its value is "123456".
	DecryptParamName string

	// test mode, ZipFile is the folder path that has unzipped files
	TestMode bool
}

// validateFlags checks the provided flag and return non nil err if any error
// found. Note that this only check for some basic things such as valid port
// range, non empty zip file name. This won't check if port is in used or
// the provided zip is valid, as it would be too length and overlap with
// other packages.
func (cfg Config) Validate() error {
	if cfg.ZipFile == "" {
		return errors.New("invalid zip file location")
	}

	port, _ := strconv.Atoi(cfg.Port)
	if port <= 1023 || port >= 65535 {
		return errors.New("invalid port, must be between 1023 and 65535")
	}
	return nil
}

type ZipSV interface {
	Start()
}

func New(cfg Config) ZipSV {
	sv := &zipsv{
		cfg: cfg,
	}
	if cfg.TestMode == false {

		var err error
		sv.fs, err = zipfs.New(cfg.ZipFile)
		if err != nil {
			log.Fatal(err)
		}
	}
	return sv
}

type zipsv struct {
	cfg Config
	fs  *zipfs.FileSystem
}

func toHTTPError(err error) (msg string, httpStatus int) {
	if pathErr, ok := err.(*os.PathError); ok {
		err = pathErr.Err
	}
	if os.IsNotExist(err) {
		return "404 page not found", http.StatusNotFound
	}
	if os.IsPermission(err) {
		return "403 Forbidden", http.StatusForbidden
	}
	// Default:
	return "500 Internal Server Error", http.StatusInternalServerError
}
func raise(sig os.Signal) error {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}
	return p.Signal(sig)
}

///DWAR API : calling /_api/v1/%cmd
/// where %cmd can be
/// 1. quit : notify the runtime to quit.
func apiHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	q := r.URL.Query()
	log.Println("path=", p)
	log.Println("query=", q)
	switch p {
	case "/_api/v1/quit":
		w.Write([]byte("quit"))
		//syscall.Kill(syscall.Getpid(), syscall.SIGINT)
		raise(os.Interrupt)
		return
	case "/_api/v1/get":
		if q != nil {
			url := q.Get("url")
			log.Println("API get url=", url)
			resp, err := http.Get(url)
			if err != nil {
				msg, code := toHTTPError(err)
				http.Error(w, msg, code)
				return
			}
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			w.Write(body)
			return
		}
	default:
		w.Write([]byte("API"))
	}
}
func (z *zipsv) Start() {
	r := chi.NewRouter()
	if z.cfg.TestMode == false {
		r.Get("/*", zipfs.FileServer(z.fs, z.cfg.DecryptParamName).ServeHTTP)
	} else {
		r.Get("/*", http.FileServer(http.Dir(z.cfg.ZipFile)).ServeHTTP)
	}
	//API end point
	r.Get("/_api/*", apiHTTP)

	listenAddress := z.cfg.Host + ":" + z.cfg.Port
	log.Printf("listening at: %v", listenAddress)
	//	log.Fatal(http.ListenAndServe(listenAddress, r))

	//graceful quit
	srv := &http.Server{Addr: listenAddress, Handler: r}
	fin := finish.New()
	fin.Add(srv)

	go func() {
		err := srv.ListenAndServe()
		if err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	fin.Wait()
}
