package main

import (
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/arasemco/stacksmith/api/internal/services"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	root := flag.String("root", "data", "directory the services are stored in")
	flag.Parse()

	store := services.NewStore(*root)
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	mux.HandleFunc("GET /v1/services", func(w http.ResponseWriter, _ *http.Request) {
		entries, err := store.List()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"services": entries})
	})
	mux.HandleFunc("POST /v1/services", func(w http.ResponseWriter, r *http.Request) {
		var spec services.Spec
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&spec); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := services.Validate(spec); err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		entry, err := store.Create(spec)
		if errors.Is(err, services.ErrExists) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		writeJSON(w, http.StatusCreated, entry)
	})
	mux.HandleFunc("GET /v1/services/{name}/files/{path...}", func(w http.ResponseWriter, r *http.Request) {
		content, err := store.ReadFile(r.PathValue("name"), r.PathValue("path"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"path": r.PathValue("path"), "content": content})
	})

	mux.HandleFunc("GET /v1/services/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		content, err := store.Read(name)
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "no such service", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		spec, err := store.Spec(name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"name":    name,
			"path":    "services/" + name + "/docker-compose.yml",
			"content": content,
			"spec":    spec,
		})
	})
	mux.HandleFunc("DELETE /v1/services/{name}", func(w http.ResponseWriter, r *http.Request) {
		if err := store.Delete(r.PathValue("name")); errors.Is(err, os.ErrNotExist) {
			http.Error(w, "no such service", http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("PUT /v1/services/{name}", func(w http.ResponseWriter, r *http.Request) {
		var spec services.Spec
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&spec); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := services.Validate(spec); err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		entry, err := store.Update(r.PathValue("name"), spec)
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "no such service", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		writeJSON(w, http.StatusOK, entry)
	})

	log.Printf("api listening on %s (root %s)", *addr, *root)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}
