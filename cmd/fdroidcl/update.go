// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/mvdan/basedir"
	"github.com/mvdan/fdroidcl"
)

var cmdUpdate = &Command{
	UsageLine: "update",
	Short:     "Update the index",
}

func init() {
	cmdUpdate.Run = runUpdate
}

func runUpdate(args []string) {
	if err := updateIndex(); err != nil {
		log.Fatalf("Could not update index: %v", err)
	}
}

func updateIndex() error {
	url := fmt.Sprintf("%s/%s", repoURL, "index.jar")
	if err := downloadEtag(url, indexPath(repoName), nil); err != nil {
		return err
	}
	return nil
}

func respEtag(resp *http.Response) string {
	etags, e := resp.Header["Etag"]
	if !e || len(etags) == 0 {
		return ""
	}
	return etags[0]
}

func downloadEtag(url, path string, sum []byte) error {
	fmt.Printf("Downloading %s... ", url)
	defer fmt.Println()
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)

	etagPath := path + "-etag"
	if _, err := os.Stat(path); err == nil {
		etag, _ := ioutil.ReadFile(etagPath)
		req.Header.Add("If-None-Match", string(etag))
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		fmt.Printf("not modified")
		return nil
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if sum == nil {
		_, err := io.Copy(f, resp.Body)
		if err != nil {
			return err
		}
	} else {
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		got := sha256.Sum256(data)
		if !bytes.Equal(sum, got[:]) {
			return errors.New("sha256 mismatch")
		}
		if _, err := f.Write(data); err != nil {
			return err
		}
	}
	if err := ioutil.WriteFile(etagPath, []byte(respEtag(resp)), 0644); err != nil {
		return err
	}
	fmt.Printf("done")
	return nil
}

func indexPath(name string) string {
	cache, err := basedir.Cache()
	if err != nil {
		log.Fatalf("Could not determine cache dir: %v", err)
	}
	return filepath.Join(appSubdir(cache), repoName+".jar")
}

func appSubdir(dir string, subdir ...string) string {
	elems := append([]string{dir, "fdroidcl"}, subdir...)
	p := filepath.Join(elems...)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.Fatalf("Could not create app dir: %v", err)
	}
	return p
}

func mustLoadIndex() *fdroidcl.Index {
	p := indexPath(repoName)
	f, err := os.Open(p)
	if err != nil {
		log.Fatalf("Could not open index file: %v", err)
	}
	stat, err := f.Stat()
	if err != nil {
		log.Fatalf("Could not stat index file: %v", err)
	}
	//pubkey, err := hex.DecodeString(repoPubkey)
	//if err != nil {
	//	log.Fatalf("Could not decode public key: %v", err)
	//}
	index, err := fdroidcl.LoadIndexJar(f, stat.Size(), nil)
	if err != nil {
		log.Fatalf("Could not load index: %v", err)
	}
	return index
}
