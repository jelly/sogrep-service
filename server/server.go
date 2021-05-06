package server

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
	"strings"
	"net/http"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"github.com/gorilla/mux"
)

var (
	sonamesMap sync.Map
)

func eraseSyncMap(m *sync.Map) {
    m.Range(func(key interface{}, value interface{}) bool {
        m.Delete(key)
        return true
    })
}

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


	go func() {
		logger.Debugln("setup watchers")
		s.SetupWatchers("")
	}()

	logger.Infoln("parsing link databases")
	// Parse link Database on startup
	start := time.Now()
	s.ParseLinksDatabases(s.config.RepositoryDirectory)
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

func (s *Server) EventHandler(ch chan string, matches int) {
	logger := s.logger
	var events []string
	timeoutDuration := time.Duration(10)

	timeout := time.After(timeoutDuration * time.Second)

	for {
		select {
		case result := <-ch:
			logger.WithField("channel msg", result).Debugln("received channel message")
			events = append(events, result);
		case <- timeout:
			timeout = time.After(timeoutDuration * time.Second)

			if len(events) == 0 {
				break
			}

			logger.WithField("events", events).Debugln("collected events")

			start := time.Now()
			s.ParseLinksDatabases(s.config.RepositoryDirectory)
			duration := time.Since(start)
			logger.WithField("duration", duration).Infoln("link databases parsed")

			// reset events
			events = make([]string, 0)
		}
	}
}

func (s *Server) SetupWatchers(location string) error {
	logger := s.logger
	// TODO: use one source of truth?
	matches := s.GetLinksDatabases(location)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.WithError(err).Errorf("failed to start inotfiy watcher")
		return err
	}

	defer watcher.Close()

	for _, match := range matches {
		watcher.Add(match)
	}

	// createlinks gives two WRITE's for one database
	changes := make(chan string)
	done := make(chan bool)

	go s.EventHandler(changes, len(matches) * 2)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				logger.WithField("event", event).Debugln("received inotify event")
				if event.Op&fsnotify.Write == fsnotify.Write {
					changes <- event.Name
				}
			case err := <-watcher.Errors:
				logger.WithError(err).Debugln("received inotify error")
			}
		}
	}()

	<-done

	return nil
}


func (s *Server) GetLinksDatabases(location string) []string {
	logger := s.logger

	pattern := fmt.Sprintf("%s/*/os/*/*.links.tar.gz", location);

	matches, err := filepath.Glob(pattern)
	if err != nil {
		logger.WithError(err).Errorf("Unable to find links databases")
	}

	return matches
}

// /srv/ftp/$repo/os/$arch/$repo.links.tar.gz
func (s *Server) ParseLinksDatabases(location string) {
	var wg sync.WaitGroup


	logger := s.logger
	matches := s.GetLinksDatabases(location)

	s.sonamesMapMutex.Lock()
	eraseSyncMap(&sonamesMap)
	for _, match := range matches {
		logger.WithField("db", match).Debug("parsing links database")
		wg.Add(1)
		go s.ParseLinksDatabase(match, &wg)
	}

	wg.Wait()
	s.sonamesMapMutex.Unlock()
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
