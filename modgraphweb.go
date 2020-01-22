package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sync"
)

const portEnvVar = "PORT"
const defaultPort = "8080"
const tempPattern = "modgraphweb_"
const maxUploadSize = 32 << 20
const svgContentType = "image/svg+xml"
const randomNameBytes = 16

// cache pages for 1 hour
const cacheControlValue = "max-age=3600"

const graphFormName = "graph"
const uploadPath = "/upload"
const rawPath = "/raw"
const viewPath = "/view/"

type server struct {
	mu   sync.Mutex
	svgs map[string][]byte
}

func cacheable(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", cacheControlValue)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("rootHandler %s %s", r.Method, r.URL.String())
	if r.Method != http.MethodGet {
		http.Error(w, "wrong method", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != "/" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	cacheable(w)
	w.Write([]byte(rootTemplate))
}

func (s *server) uploadHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("uploadHandler %s %s", r.Method, r.URL.String())
	if r.Method != http.MethodPost {
		http.Error(w, "wrong method", http.StatusMethodNotAllowed)
		return
	}
	err := s.uploadHandlerWithErr(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func randomName() string {
	b := make([]byte, randomNameBytes)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (s *server) uploadHandlerWithErr(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		return err
	}
	contents := r.Form.Get(graphFormName)
	if contents == "" {
		return errors.New("no graph contents")
	}

	path, err := s.processModGraph([]byte(contents))
	if err != nil {
		return err
	}
	http.Redirect(w, r, path, http.StatusSeeOther)
	return nil
}

// Returns the path for the svg, or an error.
func (s *server) processModGraph(b []byte) (string, error) {
	log.Printf("executing modgraphviz on %d bytes...", len(b))
	modgraphvizCmd := exec.Command("modgraphviz")
	modgraphvizCmd.Stdin = bytes.NewReader(b)
	modgraphvizCmd.Stderr = os.Stderr
	dotBytes, err := modgraphvizCmd.Output()
	if err != nil {
		return "", err
	}

	log.Printf("executing dot on %d bytes from modgraphviz ...", len(dotBytes))
	dotCmd := exec.Command("dot", "-Tsvg")
	dotCmd.Stdin = bytes.NewReader(dotBytes)
	dotCmd.Stderr = os.Stderr
	svgBytes, err := dotCmd.Output()
	if err != nil {
		return "", err
	}

	name := randomName()
	log.Printf("storing %d bytes of svg with name %s", len(svgBytes), name)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.svgs[name] = svgBytes

	return viewPath + name, nil
}

func (s *server) rawHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("rawHandler %s %s", r.Method, r.URL.String())
	if r.Method != http.MethodPost {
		http.Error(w, "wrong method", http.StatusMethodNotAllowed)
		return
	}
	err := s.rawHandlerWithErr(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *server) rawHandlerWithErr(w http.ResponseWriter, r *http.Request) error {
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	err = r.Body.Close()
	if err != nil {
		return err
	}

	path, err := s.processModGraph(bodyBytes)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/plain;charset=utf-8")
	// TODO: detect from headers
	r.URL.Scheme = "https"
	r.URL.Host = r.Host
	r.URL.Path = path
	_, err = fmt.Fprintf(w, "Open: %s\n", r.URL.String())
	return err
}

var nameRegexp = regexp.MustCompile("^" + viewPath + "([^/]+)$")

func (s *server) serveSVG(w http.ResponseWriter, r *http.Request) {
	log.Printf("serveSVG %s %s", r.Method, r.URL.String())
	if r.Method != http.MethodGet {
		http.Error(w, "wrong method", http.StatusMethodNotAllowed)
		return
	}
	matches := nameRegexp.FindStringSubmatch(r.URL.Path)
	if len(matches) != 2 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	name := matches[1]

	s.mu.Lock()
	defer s.mu.Unlock()

	svgBytes := s.svgs[name]
	if svgBytes == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	cacheable(w)
	w.Header().Set("Content-Type", svgContentType)
	_, err := w.Write(svgBytes)
	if err != nil {
		panic(err)
	}
}

func main() {
	s := &server{sync.Mutex{}, make(map[string][]byte)}

	fmt.Println("PATH", os.Getenv("PATH"))
	mux := http.NewServeMux()
	mux.HandleFunc("/", rootHandler)
	mux.HandleFunc(uploadPath, s.uploadHandler)
	mux.HandleFunc(rawPath, s.rawHandler)
	mux.HandleFunc(viewPath, s.serveSVG)

	port := os.Getenv(portEnvVar)
	if port == "" {
		port = defaultPort
		log.Printf("warning: %s not specified; using default %s", portEnvVar, port)
	}

	addr := ":" + port
	log.Printf("listen addr %s (http://localhost:%s/)", addr, port)
	if err := http.ListenAndServe(addr, mux); err != nil {
		panic(err)
	}
}

const rootTemplate = `<!doctype html>
<html>
<head><title>modgraphviz Web Interface</title></head>
<body>
<h1>modgraphviz Web Interface</h1>
<p>Runs <a href="https://godoc.org/golang.org/x/exp/cmd/modgraphviz">modgraphviz</a> on the web and produces an SVG. Paste the contents of <code>go mod graph</code> below, then either save the resulting SVG or share the link.</p>

<p>Single line: <code>go mod graph | curl --data-binary '@-' http://localhost:8080/raw</code></p>

<form method="post" action="` + uploadPath + `" enctype="multipart/form-data">
<textarea rows="40" cols="120" name="` + graphFormName + `">
</textarea>

<p><input type="submit" value="Upload"></p>
</form>
</body>
</html>
`
