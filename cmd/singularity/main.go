package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Singularity artifact file server: stores qcow2 images and other large
// artifacts produced by Supernova, and serves them (plus sha256 checksums)
// to nodes over plain HTTP. Complements the Zot OCI registry which handles
// container images only.

func main() {
	dataDir := envOr("ARTIFACTS_DIR", "/data/artifacts")
	addr := ":" + envOr("ARTIFACTS_PORT", "8080")

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /artifacts", listHandler(dataDir))
	mux.HandleFunc("GET /artifacts/{name}", getHandler(dataDir))
	mux.HandleFunc("GET /artifacts/{name}/sha256", checksumHandler(dataDir))
	mux.HandleFunc("PUT /artifacts/{name}", putHandler(dataDir))

	log.Printf("singularity artifacts listening on %s (dir %s)", addr, dataDir)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func safePath(dataDir, name string) (string, error) {
	if strings.Contains(name, "/") || strings.Contains(name, "..") || name == "" {
		return "", fmt.Errorf("invalid artifact name %q", name)
	}
	return filepath.Join(dataDir, name), nil
}

func putHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		dst, err := safePath(dataDir, name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		tmp := dst + ".tmp"
		f, err := os.Create(tmp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		h := sha256.New()
		if _, err := io.Copy(io.MultiWriter(f, h), r.Body); err != nil {
			f.Close()
			os.Remove(tmp)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := f.Close(); err != nil {
			os.Remove(tmp)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := os.Rename(tmp, dst); err != nil {
			os.Remove(tmp)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		sum := "sha256:" + hex.EncodeToString(h.Sum(nil))
		if err := os.WriteFile(dst+".sha256", []byte(sum), 0o644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("stored artifact %s (%s)", name, sum)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"name":%q,"checksum":%q}`+"\n", name, sum)
	}
}

func getHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dst, err := safePath(dataDir, r.PathValue("name"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.ServeFile(w, r, dst)
	}
}

func checksumHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dst, err := safePath(dataDir, r.PathValue("name"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sum, err := os.ReadFile(dst + ".sha256")
		if err != nil {
			http.Error(w, "checksum not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write(sum)
	}
}

func listHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := os.ReadDir(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		names := []string{}
		for _, e := range entries {
			if !e.IsDir() && !strings.HasSuffix(e.Name(), ".sha256") && !strings.HasSuffix(e.Name(), ".tmp") {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "[")
		for i, n := range names {
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			fmt.Fprintf(w, "%q", n)
		}
		fmt.Fprint(w, "]\n")
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
