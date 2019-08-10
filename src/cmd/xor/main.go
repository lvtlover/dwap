package main

import (
	"io"
	"log"
	"os"
	"zipsv/internal/zipfs"

	"github.com/spf13/pflag"
)

func main() {
	var key string
	var inName string
	pflag.StringVarP(&key, "key", "e", "", "xor key")
	pflag.StringVarP(&inName, "in", "i", "", "input file name")
	pflag.Parse()

	if key == "" {
		log.Fatal("please provide a secret")
		return
	}

	if inName == "" {
		log.Fatal("please provide input file")
		return
	}

	in, err := os.Open(inName)
	if err != nil {
		log.Fatalf("fail to open file: %v", err)
		return
	}

	info, err := in.Stat()

	if err != nil {
		log.Fatalf("fail to read file stat: %v", err)
		return
	}

	if info.IsDir() {
		log.Fatalf("expected a file, not a dir")
		return
	}

	var out io.Writer
	if out, err = os.Create(inName + ".xor"); err != nil {
		log.Fatalf("fail to create out file: %v", err)
		return
	}

	_, err = io.Copy(out, zipfs.XorTransformer(in, key))
	if err != nil {
		log.Fatalf("fail to encrypt: %v", err)
	}
}
