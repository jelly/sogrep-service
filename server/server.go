package server

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"
	"strings"
	"syscall"
	"net"
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
	var wg sync.WaitGroup

	_, serveCtxCancel := context.WithCancel(ctx)
	defer serveCtxCancel()

	logger := s.logger

	// Obtain links databases
	matches, err := s.GetLinksDatabases(s.config.RepositoryDirectory)
	if err != nil {
		logger.WithError(err).Errorf("Unable to find links databases")
		return err
	}

	if matches == nil {
		return errors.New("server: no links databases found")
	}

	logger.Infoln("parsing link databases")
	// Parse link Database on startup
	start := time.Now()
	s.ParseLinksDatabases(matches)
	duration := time.Since(start)

	logger.WithField("duration", duration).Infoln("link databases parsed")

	errCh := make(chan error, 2)
	exitCh := make(chan bool, 1)
	signalCh := make(chan os.Signal, 1)
	inotifyDone := make(chan bool)

	listener, err := net.Listen("tcp", s.config.ListenAddress)
	if err != nil {
		logger.WithError(err).Errorf("failed to create http socket")
		return err
	}
	defer listener.Close()

	router := mux.NewRouter().StrictSlash(true)
    router.HandleFunc("/{soname}", s.handleSonameRequest)

	sogrepServer := http.Server{
		Handler: router,
	}

	wg.Add(1)

	go func() {
		defer wg.Done()

		err := sogrepServer.Serve(listener)
		if err != nil {
			errCh <- err
		}

		logger.Debugln("http listener stopped")
	}()

	wg.Add(1)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.WithError(err).Errorf("failed to start inotfiy watcher")
		return err
	}

	go func() {
		defer wg.Done()

		logger.Debugln("setup watchers")
		err = s.SetupWatchers(watcher, matches, inotifyDone)
		if err != nil {
			errCh <- err
		}

		logger.Debugln("inotify listener stopped")
	}()

	// Wait for exit or error.
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err = <-errCh:
		// breaks
	case reason := <-signalCh:
		logger.WithField("signal", reason).Warnln("received signal")
		// breaks
	}

	logger.Infoln("clean server shutdown start")

	shutDownCtx, shutDownCtxCancel := context.WithTimeout(ctx, 10*time.Second)
	go func() {
		if shutdownErr := sogrepServer.Shutdown(shutDownCtx); shutdownErr != nil {
			logger.WithError(shutdownErr).Warn("clean http server shutdown failed")
		}
	}()

	_, shutDownCtxCancel2 := context.WithTimeout(ctx, 10*time.Second)
	go func() {
		inotifyDone <- true
		if shutdownErr := watcher.Close(); shutdownErr != nil {
			logger.WithError(shutdownErr).Warn("clean http server shutdown failed")
		}
	}()

	go func() {
		wg.Wait()
		close(exitCh)
	}()

	// Cancel our own context,
	serveCtxCancel()
	func() {
		for {
			select {
			case <-exitCh:
				return
			default:
				// HTTP listener has not quit yet.
				logger.Info("waiting for listeners to exit")
			}
			select {
			case reason := <-signalCh:
				logger.WithField("signal", reason).Warn("received signal")
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	}()

	shutDownCtxCancel()  // prevent leak.
	shutDownCtxCancel2() // prevent leak.

	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}

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

func (s *Server) EventHandler(ch chan string, matches []string) {
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
			s.ParseLinksDatabases(matches)
			duration := time.Since(start)
			logger.WithField("duration", duration).Infoln("link databases parsed")

			// reset events
			events = make([]string, 0)
		}
	}
}

func (s *Server) SetupWatchers(watcher *fsnotify.Watcher, matches []string, done chan bool) error {
	logger := s.logger

	for _, match := range matches {
		watcher.Add(match)
	}

	changes := make(chan string)

	go s.EventHandler(changes, matches)

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


func (s *Server) GetLinksDatabases(location string) ([]string, error) {
	pattern := fmt.Sprintf("%s/*/os/*/*.links.tar.gz", location);

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return matches, err
	}

	return matches, nil
}

// /srv/ftp/$repo/os/$arch/$repo.links.tar.gz
func (s *Server) ParseLinksDatabases(matches []string) {
	var wg sync.WaitGroup

	logger := s.logger

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
