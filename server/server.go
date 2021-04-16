package server

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
	"strings"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/gorilla/mux"
)

var (
	sonamesMap sync.Map
)

type Server struct {
        config *Config

        logger logrus.FieldLogger

		sonamesMapMutex sync.RWMutex
}

type Empty struct {
}

type Soname struct {
	Packages []string `json:"packages"`
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
	start := time.Now()
	s.ParseLinksDatabases("")
	duration := time.Since(start)

	logger.WithField("duration", duration).Infoln("link databases parsed")

	router := mux.NewRouter().StrictSlash(true)
    router.HandleFunc("/{soname}", s.handleSonameRequest)

	http.ListenAndServe(":8080", router)

	return err
}

func (s *Server) handleSonameRequest(w http.ResponseWriter, r *http.Request) {
	logger := s.logger
	vars := mux.Vars(r)
	soname := vars["soname"]

	logger.WithField("soname", soname).Debugln("handle soname request")

	s.sonamesMapMutex.RLock()
	packages, ok := sonamesMap.Load(soname)
	s.sonamesMapMutex.RUnlock()

	if ! ok {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(Empty{})
		return
	}

	object := Soname{packages.([]string)}

	json.NewEncoder(w).Encode(object)
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
