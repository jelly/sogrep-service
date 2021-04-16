package server

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"strings"

	"github.com/sirupsen/logrus"
)

var (
        sonamesMap sync.Map
)

// Server is our HTTP server implementation.
type Server struct {
        config *Config

        logger logrus.FieldLogger
}

func NewServer(c *Config) (*Server, error) {
        s := &Server{
                config: c,
                logger: c.Logger,
        }

        return s, nil
}

func (s *Server) Serve(ctx context.Context) error {
	var err error

	_, serveCtxCancel := context.WithCancel(ctx)
	defer serveCtxCancel()

	logger := s.logger

	logger.Infoln("parsing link databases")

	// Parse link Database on startup
	s.ParseLinksDatabases("")

	logger.Infoln("link databases parsed")

	return err
}

// /srv/ftp/$repo/os/$arch/$repo.links.tar.gz
func (s *Server) ParseLinksDatabases(location string) {
	var wg sync.WaitGroup

	logger := s.logger
	pattern := "./ftp/*/os/*/*.links.tar.gz"

	matches, err := filepath.Glob(pattern)
	if err != nil {
		logger.WithError(err).Errorf("Unable to find links databases")
	}

	for _, match := range matches {
		logger.WithField("db", match).Debug("parsing links database")
		wg.Add(1)
		go s.ParseLinksDatabase(match, &wg)
	}

	wg.Wait()

	value, _ := sonamesMap.Load("libavif.so")
	logger.Infoln(value)
}

func (s *Server) ParseLinksDatabase(file string, wg *sync.WaitGroup) error {
	logger := s.logger

	defer wg.Done()

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

		data := make([]byte, header.Size)
		n, err2 := archive.Read(data)

		if err2 != io.EOF && err2 != nil {
			logger.WithError(err2).Warn("lala")
			continue
		}

		if n <= 0 {
			logger.WithField("filename", filename).Warnln("empty file")
			continue
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

	return nil
}
