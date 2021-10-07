package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/linuskmr/logo"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
)

// fileExists checks whether filename exists or not.
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return true
}

type CdnServer struct {
	// CachePath is the path to the directory where the CDN server saves cached files
	// from the root server.
	CachePath string
	// RootServer is the URL of the root server to load data from.
	RootServer string
}

func (c *CdnServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logo.Print("Request to", r.URL.Path)

	if fileExists(c.pathToCache(r.URL.Path)) {
		// File exists in local cache, so serve that file
		logo.Debug("File exists in local cache, so serve that file")
		http.ServeFile(w, r, c.pathToCache(r.URL.Path))
		return
	}

	// File not in local cache. Request that file from the root server
	logo.Debug("File not in local cache. Request that file from the root server")
	err := c.loadFromRootServer(w, r.URL.Path)
	if err != nil {
		return
	}

	// File now in local cache. Now serve that file from cache
	logo.Debug("File now in local cache. Now serve that file from cache")
	http.ServeFile(w, r, c.pathToCache(r.URL.Path))
}

// pathToCache returns the absolute path to the requestPath in the cache from this server.
func (c *CdnServer) pathToCache(requestPath string) string {
	return path.Join(c.CachePath, requestPath)
}

// serveFromCache loads the file requested by requestPath from cache and writes
// its content to w. If the file does not exist in the cache, false is returned.
/*func (h *CdnServer) serveFromCache(w http.ResponseWriter, requestPath string) bool {
	file, err := os.Open(h.pathToCache(requestPath))
	if err != nil {
		// File not found in cache
		return false
	}

	cors(w)
	fileReader := bufio.NewReader(file)
	_, err = fileReader.WriteTo(w)
	if err != nil {
		logo.Print("Could not read", h.pathToCache(requestPath), "or write to response:", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Could not read", requestPath, "or write to response:", err)
		return true
	}
	return true
}*/

/*func cors(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")
}*/

// loadFromRootServer loads the file specified by requestPath from the root
// server and saves it in the cache of the CdnServer.
func (c *CdnServer) loadFromRootServer(w http.ResponseWriter, requestPath string) error {
	// Build a get request to the root server
	request, err := http.NewRequest(http.MethodGet, c.RootServer+requestPath, nil)
	if err != nil {
		logo.Error("Could not create http request to root server", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Could not create http request to root server")
		return err
	}
	// Set the X-Cdn-Request so that the root server does not redirect us to another cdn server
	request.Header.Set("X-Cdn-Request", "true")

	// Make the request
	httpClient := &http.Client{}
	response, err := httpClient.Do(request)
	if err != nil {
		// File not found on root server -> return 404 not found
		logo.Debug("Could not find", requestPath, "on root server. Responding with 404 not found:", err)
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(w, "Could not find", requestPath, "on root server:", err)
		return err
	}

	// Open file in local cache to write the response from the root server to
	file, err := os.OpenFile(c.pathToCache(requestPath), os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		logo.Error("Could not create cache file for", requestPath, ":", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Could not create cache file for", requestPath, ":", err)
		return err
	}

	// Write response from root server to cache file
	_, err = bufio.NewReader(response.Body).WriteTo(file)
	if err != nil {
		logo.Error("Could not write cache file for", requestPath, ":", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Could not write cache file", requestPath, ":", err)
		return err
	}
	return nil
}

// createTmpCache creates a temporary dictionary for caching files from the root server.
func createTmpCache() string {
	tmpCache, err := ioutil.TempDir("", "distrihttp_data")
	if err != nil {
		logo.Error("Could not create tmp directory for storing cached data:", err.Error())
		panic(err)
	}
	logo.Info("Data cache location is", tmpCache)
	return tmpCache
}

func quote(v interface{}) string {
	return fmt.Sprintf("\"%v\"", v)
}

/// registerAtRootServer registers this CdnServer at its root server by sending a post request to it.
func (c *CdnServer) registerAtRootServer(addr string) {
	_, err := http.Post(c.RootServer+"/cdn_register", "", strings.NewReader(addr))
	if err != nil {
		logo.Fatal("Registration at root server", quote(c.RootServer), "failed:", err)
	} else {
		logo.Info("Successful registered at root server", quote(c.RootServer))
	}
}

// ListenAndServe registers the CdnServer at its root server with remoteAddr, calls
// http.ListenAndServe(addr, CdnServer) and panics on errors of http.ListenAndServe().
func (c *CdnServer) ListenAndServe(listenAddr string, remoteAddr string) {
	c.registerAtRootServer(remoteAddr)
	logo.Info("Listening on", listenAddr)
	err := http.ListenAndServe(listenAddr, c)
	if err != nil {
		logo.Fatal("http.ListenAndServe():", err)
	}
}

// shutdown deletes the cache path of the server.
func (c *CdnServer) shutdown() {
	err := os.RemoveAll(c.CachePath)
	if err != nil {
		logo.Error("Deleted temporary cache", c.CachePath, "failed:", err)
	} else {
		logo.Debug("Deleted temporary cache", c.CachePath)
	}
}

func main() {
	logo.Default.Funcname = false
	logo.Default.DateFormat = ""

	listenAddr := flag.String("listen-addr", ":8193", "Address where the this CDN server should listen")
	remoteAddr := flag.String("remote-addr", "http://localhost:8193", "The address this server is reachable at for clients redirected by the root server")
	rootAddr := flag.String("root-addr", "http://localhost:8192", "Address of the root server that should be mirrored")
	flag.Parse()
	logo.Debug("Config:")
	logo.Debug("  listenAddr:", quote(*listenAddr))
	logo.Debug("  remoteAddr:", quote(*remoteAddr))
	logo.Debug("  rootAddr:", quote(*rootAddr))

	tmpCache := createTmpCache()
	cdnServer := CdnServer{
		CachePath:  tmpCache,
		RootServer: *rootAddr,
	}
	defer cdnServer.shutdown()

	go cdnServer.ListenAndServe(*listenAddr, *remoteAddr)

	// Listen for interrupt or termination signal
	signalChan := make(chan os.Signal)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	exitSignal := <-signalChan // Wait for a termination signal

	logo.Info("Received", exitSignal, "from os. Shutting down")
}
