package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"dwap/internal/zipsv"

	"github.com/spf13/pflag"
)

const (
	// default port to for web server to listen to
	defaultServerPort = "8080"

	// default host for the server to to bind to
	defaultServerHostBinding = ""

	// the key name to be added on the URL, for example;
	// "localhost:8080/a.js?key=123456", here, the key name is "key",
	// and, its value is "123456".
	defaultEncryptionParam = "key"

	gVersion = "0.1"
)

func main() {
	cfg := defaultConfig()
	bindConfig(&cfg)

	showHelp := false
	pflag.BoolVarP(&showHelp, "help", "h", showHelp, "show help")

	showVersion := false
	pflag.BoolVarP(&showVersion, "version", "v", showVersion, "show version")

	pflag.Parse()

	if showVersion {
		fmt.Println(gVersion)
		return
	}

	if showHelp || pflag.NArg() == 0 {
		printUsage()
		return
	}

	cfg.ZipFile = pflag.Arg(0)
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}
	openbrowser("http://127.0.0.1:" + cfg.Port)
	zipsv.New(cfg).Start()
}

func printUsage() {
	_, _ = fmt.Fprintf(os.Stderr, `Distributed Web Application (dwpp) Runner.

Usage: 
	%s [options] file/dir
where:
	file: file ended with dwap extension
	dir: when -t is used
Options: 
`, os.Args[0])
	pflag.PrintDefaults()
}

func findport() string {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	return strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
}

func defaultConfig() zipsv.Config {
	cfg := zipsv.Config{
		Port:             defaultServerPort,
		Host:             defaultServerHostBinding,
		DecryptParamName: defaultEncryptionParam,
		TestMode:         false,
	}
	cfg.Port = findport()
	return cfg
}

func bindConfig(cfg *zipsv.Config) {
	// common flag, hence, use VarP types to enable short form
	//	pflag.StringVarP(&cfg.ZipFile, "zip-file", "z", "", "zip file to serve")
	pflag.StringVarP(&cfg.Port, "port", "p", cfg.Port, "server port")
	pflag.StringVarP(&cfg.DecryptParamName, "key", "k", cfg.DecryptParamName, "encryption param name")
	pflag.BoolVarP(&cfg.TestMode, "test", "t", cfg.TestMode, "test mode, use folder instead of file")
	// uncommon flag, only provide long form
	//pflag.StringVar(&cfg.Host, "host", cfg.Host, "host address to bind to, set to 0.0.0.0 to allow local network")
}

func openbrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Fatal(err)
	}

}
