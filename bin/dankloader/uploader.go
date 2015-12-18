package main

import (
	"fmt"
	"github.com/levenlabs/dank/dank-client"
	"github.com/mediocregopher/lever"
	"log"
	"os"
	"path/filepath"
	"sync"
)

func main() {
	l := lever.New("dankloader", nil)
	l.Add(lever.Param{
		Name:        "--dank-addr",
		Description: "address:port of the dank instance to upload to",
		Default:     "127.0.0.1:8333",
	})
	l.Add(lever.Param{
		Name:        "--concurrent",
		Description: "number of concurrent uploads",
		Default:     "4",
	})
	l.Parse()

	dankURL, _ := l.ParamStr("--dank-addr")
	con, _ := l.ParamInt("--concurrent")

	c := dank.NewClient(dankURL)

	ac := len(os.Args)
	if ac < 2 {
		log.Fatal("Usage: dankloader [folder or file to upload]")
	}
	fName := os.Args[ac-1]
	f, err := os.Open(fName)
	if err != nil {
		log.Fatalf("Error reading %s: %s", fName, err)
	}
	defer f.Close()
	fs, err := f.Stat()
	if err != nil {
		log.Fatalf("Error reading %s: %s", fName, err)
	}

	var files []string
	if fs.IsDir() {
		files = []string{}
		filepath.Walk(fName, func(p string, i os.FileInfo, err error) error {
			if err != nil {
				log.Fatalf("Error reading %s: %s", p, err)
			}
			if i.IsDir() {
				return nil
			}
			files = append(files, p)
			return nil
		})
	} else {
		files = []string{fName}
	}

	var wg sync.WaitGroup
	ch := make(chan string)
	for i := 0; i < con; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range ch {
				r, err := makeDank(f, c)
				if err != nil {
					fmt.Printf("error uploading %s: %s", f, err)
				}
				fmt.Printf("%s => %s\n", f, r)
			}
		}()
	}
	for _, n := range files {
		ch <- n
	}
	close(ch)
	wg.Wait()
}

func makeDank(f string, c *dank.Client) (string, error) {
	return c.UploadFile(f, "")
}
