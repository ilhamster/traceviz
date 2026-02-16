package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/ilhamster/traceviz/logviz/service"
)

var (
	port         = flag.Int("port", 7410, "Port to serve LogViz clients on")
	resourceRoot = flag.String("resource_root", "", "Deprecated; use --angular_root")
	angularRoot  = flag.String("angular_root", "", "The path to the LogViz Angular client resources")
	reactRoot    = flag.String("react_root", "", "The path to the LogViz React client resources")
	logRoot      = flag.String("log_root", ".", "The root path for visualizable logs")
)

func main() {
	flag.Parse()

	if *angularRoot == "" && *resourceRoot != "" {
		*angularRoot = *resourceRoot
	}

	service, err := service.New(*angularRoot, *logRoot, 10)
	if err != nil {
		log.Fatalf("Failed to create LogViz service: %s", err)
	}

	mux := http.DefaultServeMux
	service.RegisterHandlers(mux)

	if *angularRoot != "" {
		mux.Handle("/angular/", http.StripPrefix("/angular/", http.FileServer(http.Dir(*angularRoot))))
		mux.HandleFunc("/angular", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/angular/", http.StatusFound)
		})
	}
	if *reactRoot != "" {
		mux.Handle("/react/", http.StripPrefix("/react/", http.FileServer(http.Dir(*reactRoot))))
		mux.HandleFunc("/react", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/react/", http.StatusFound)
		})
	}
	if *angularRoot != "" {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, "/angular/", http.StatusFound)
		})
	} else if *reactRoot != "" {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, "/react/", http.StatusFound)
		})
	}
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Failed to get hostname: %s", err)
	}

	basePath := "/"
	if *angularRoot != "" {
		basePath = "/angular/"
	} else if *reactRoot != "" {
		basePath = "/react/"
	}

	// Provide OSC 8 (https://en.wikipedia.org/wiki/ANSI_escape_code#OSC) link for
	// compatible terminals.
	fmt.Printf("Serving LogViz at \x1B]8;;http://%[1]s:%[2]d%[3]s\x07http://%[1]s:%[2]d%[3]s\x1B]8;;\x07", hostname, *port, basePath)
	http.ListenAndServe(
		fmt.Sprintf(":%d", *port),
		mux,
	)
}
