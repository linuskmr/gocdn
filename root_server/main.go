package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/linuskmr/logo"
	"math/rand"
	"net/http"
	"os"
	"path"
	"strings"
)

type RootServer struct {
	// ServeDir is the directory that should be served by RootServer.
	ServeDir string
	// cdnServers is a list of all registered cdn servers of this root server.
	cdnServers []string
	// SelfServedFileTypes is a list of file extensions that always should be served
	// by the root server itself and should not get propagated to a cdn server.
	// Usually you want that the root server serves .html files by itself, so that
	// the clients web browser does not get redirected to a cdn server, which causes
	// the url bar in the clients web browser to show the url of the cdn server,
	// which will confuse users.
	SelfServedFileTypes []string
}

// handleCdnConnect handles the connection request from a cdn server.
func (s *RootServer) handleCdnConnect(w http.ResponseWriter, r *http.Request) {
	// Read body from request
	var bodyBuffer strings.Builder
	bufio.NewReader(r.Body).WriteTo(&bodyBuffer)
	newCdnServer := bodyBuffer.String()
	logo.Info("Added", newCdnServer, "as CDN server")
	s.cdnServers = append(s.cdnServers, newCdnServer)
}

// fileIsDir checks whether dir exists and is a directory.
func fileIsDir(dir string) bool {
	stat, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return false
	}
	return stat.IsDir()
}

// serveFileMyself determines if file should be served by itself. Returns true if
// the file should be served by itself or false if a cdn server can serve this
// file. Files that should be served by the root server itself include
// directories and all files types listed in RootServer.SelfServedFileTypes.
func (s *RootServer) serveFileMyself(file string) bool {
	if fileIsDir(file) {
		logo.Debug("is dir")
		return true
	}
	for _, filetype := range s.SelfServedFileTypes {
		if strings.HasSuffix(file, filetype) {
			logo.Debug("has suffix", filetype)
			return true
		}
	}
	return false
}

func (s *RootServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/cdn_register" && r.Method == http.MethodPost {
		s.handleCdnConnect(w, r)
		return
	}

	logo.Info("Request to", r.URL.Path)

	cdnRequest := len(r.Header["X-Cdn-Request"]) > 0 // Check if a cdn server requests this file
	cdnServersAvailable := len(s.cdnServers) > 0
	serveFiletypeMyself := s.serveFileMyself(r.URL.Path)
	if cdnRequest || !cdnServersAvailable || serveFiletypeMyself {
		// Serve file myself
		logo.Debug(fmt.Sprintf("Serve file myself, because cdnRequest=%v cdnServersAvailable=%v, serveFiletypeMyself()=%v", cdnRequest, cdnServersAvailable, serveFiletypeMyself))
		http.ServeFile(w, r, s.absolutePath(r.URL.Path))
	} else {
		// Let a random cdn server serve the request
		cdnServer := s.randomCdnServer()
		logo.Debug("Redirecting to cdn server", cdnServer)
		http.Redirect(w, r, cdnServer+r.URL.Path, http.StatusTemporaryRedirect)
	}

	/*var fileContent bytes.Buffer
	_, err = fileReader.WriteTo(&fileContent)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Could not read", requestPath, ":", err)
		return
	}

	fileHash := sha256.Sum256(fileContent.Bytes())
	fileHashBase64 := base64.StdEncoding.EncodeToString(fileHash[:])

	w.Header().Add("Content-Type", "text/plain")
	fmt.Fprintln(w, "Hash", fileHashBase64)*/
}

// randomCdnServer selects a random cdn server from the list of available servers in RootServer.cdnServers.
func (s *RootServer) randomCdnServer() string {
	randomIndex := rand.Intn(len(s.cdnServers))
	return s.cdnServers[randomIndex]
}

// absolutePath converts a requestPath to an absolute file path in RootServer.ServeDir.
func (s *RootServer) absolutePath(requestPath string) string {
	return path.Join(s.ServeDir, requestPath)
}

func (s *RootServer) ListenAndServe(addr string) {
	logo.Info("Listening on", addr)
	err := http.ListenAndServe(addr, s)
	if err != nil {
		logo.Fatal("http.ListenAndServe():", err)
	}
}

func quote(v interface{}) string {
	return fmt.Sprintf("\"%v\"", v)
}

func main() {
	addr := flag.String("addr", ":8192", "Address of this root server")
	serveDir := flag.String("serve-dir", ".", "Filesystem path to be served")
	selfServedFileTypesStr := flag.String(
		"self-serve", "",
		"Comma seperated list of file types that should be served by the root server itself. Usually you want"+
			"that the root server serves .html files by itself, so that the clients web browser does not get redirected"+
			"to a cdn server, which causes the url bar in the clients web browser to show the url of the cdn server,"+
			"which will confuse users.",
	)
	flag.Parse()
	var selfServedFileTypes []string
	if *selfServedFileTypesStr != "" {
		selfServedFileTypes = strings.Split(*selfServedFileTypesStr, ",")
	}
	logo.Debug("Config:")
	logo.Debug("  addr:", quote(*addr))
	logo.Debug("  serveDir:", quote(*serveDir))
	logo.Debug("  selfServedFileTypes:", quote(selfServedFileTypes))

	rootServer := RootServer{
		ServeDir:            *serveDir,
		SelfServedFileTypes: selfServedFileTypes,
	}
	rootServer.ListenAndServe(*addr)
}
