/*
 * SPDX-License-Identifier: MIT
 * Copyright 2020 Jelle van der Waa <jelle@vdwaa.nl>
 */

package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"strings"
)

var (
	sonamesMap sync.Map
)

func parseLinksDB(file string) error {
	fp, err := os.Open(file)
	if err != nil {
		return err
	}

	defer fp.Close()

	gzfp, err := gzip.NewReader(fp)
	if err != nil {
		return err
	}

	archive := tar.NewReader(gzfp)

	//sonamesMap = sync.Map{}

	for {
		header, err := archive.Next()
		if err == io.EOF {
			break // End of archive
		}

		if err != nil {
			panic(err) // Or break?
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		// for example ./curl-7.76.1-1/links
		filename := filepath.Dir(header.Name)

		// Last two parts should be $pkgver, $pkrel
		filenameParts := strings.Split(filename, "-")

		pkgname := strings.Join(filenameParts[:len(filenameParts)-2], "-")

		fmt.Println(pkgname)
		fmt.Printf("Contents of %s:\n", header.Name)

		data := make([]byte, header.Size)
		n, err2 := archive.Read(data)

		if err2 != io.EOF && err2 != nil {
			fmt.Println("error", err)
			continue
		}

		if n <= 0 {
			fmt.Println("empty file")
		}

		stringData := string(data)
		sonames := strings.Split(stringData, "\n");

		for _, versionedSoname := range sonames {
			parts := strings.SplitAfter(versionedSoname, ".so")
			soname := parts[0]
			value, ok := sonamesMap.Load(soname)
			if ok {
				// Append
				value = append(value.([]string), pkgname)
				sonamesMap.Store(soname, value)
			} else {
				// Insert
				val := []string{pkgname}
				sonamesMap.Store(soname, val)
			}
		}

	}

	sonamesMap.Range(func(key interface{}, value interface{}) bool {
		//fmt.Println("key", key)
		//fmt.Println(value)
		return true
	})

	value, _ := sonamesMap.Load("libavif.so")
	fmt.Println(value)

	return nil
}

func main() {
	// /srv/ftp/$repo/os/$arch/$repo.links.tar.gz
	pattern := "./ftp/*/os/*/*.links.tar.gz"

	matches, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Println("no matches found")
	}

	for _, match := range matches {
		fmt.Println(match)
		parseLinksDB(match)
	}
}
