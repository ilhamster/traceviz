package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/ilhamster/traceviz/logviz/service"
)

var (
	port = flag.Int("port", 7410, "Port to serve LogViz clients on")
	// Deprecated; use --angular_root.
	resourceRoot = flag.String("resource_root", "", "Deprecated; use --angular_root")
	angularRoot  = flag.String("angular_root", "", "The path to the LogViz Angular client resources")
	reactRoot    = flag.String("react_root", "", "The path to the LogViz React client resources")
	logRoot      = flag.String("log_root", ".", "The root path for visualizable logs")
	// Optional long-running shell command for frontend watch builds.
	// React example:
	//   --client_watch_cwd .. --client_watch_cmd "pnpm --filter ./logviz/react-client exec vite build --watch"
	clientWatchCmd = flag.String("client_watch_cmd", "", "Optional shell command to run a frontend watch build; terminated when the server exits")
	clientWatchCWD = flag.String("client_watch_cwd", "", "Working directory for --client_watch_cmd (defaults to current working directory)")
)

func main() {
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *angularRoot == "" && *resourceRoot != "" {
		*angularRoot = *resourceRoot
	}

	service, err := service.New(*angularRoot, *logRoot, 10)
	if err != nil {
		log.Fatalf("Failed to create LogViz service: %s", err)
	}
	var watchDone chan error
	if *clientWatchCmd != "" {
		watchProcess := exec.CommandContext(ctx, "sh", "-c", *clientWatchCmd)
		if *clientWatchCWD != "" {
			watchProcess.Dir = *clientWatchCWD
		}
		watchProcess.Stdout = os.Stdout
		watchProcess.Stderr = os.Stderr
		watchProcess.Stdin = os.Stdin
		log.Printf("Starting client watch command: %q", *clientWatchCmd)
		if err := watchProcess.Start(); err != nil {
			log.Fatalf("Failed to start client watch command: %s", err)
		}
		watchDone = make(chan error, 1)
		go func() {
			watchDone <- watchProcess.Wait()
			close(watchDone)
		}()
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
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: mux,
	}
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.ListenAndServe()
	}()

	select {
	case err := <-serverDone:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server exited with error: %s", err)
		}
	case <-ctx.Done():
	}

	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("Server shutdown error: %s", err)
	}
	if watchDone != nil {
		if err, ok := <-watchDone; ok && err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("Client watch command exited with error: %s", err)
		}
	}
}
