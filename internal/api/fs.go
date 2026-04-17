package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Folder picker minimal pour l'UI : on expose la liste des
// sous-dossiers d'un chemin absolu et le $HOME du user.
//
// Restriction : on refuse de sortir du HOME sauf si le caller
// demande explicitement un path absolu (dans ce cas il sait ce
// qu'il fait). La UI commence par /api/v1/fs/home puis descend
// avec /api/v1/fs/list.

type fsEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"` // absolu, prêt à ré-injecter en query param
	IsDir bool   `json:"is_dir"`
}

type fsListResponse struct {
	Path    string    `json:"path"`
	Parent  string    `json:"parent,omitempty"` // "" quand on est à la racine du FS
	Home    string    `json:"home"`
	Entries []fsEntry `json:"entries"`
}

// handleFSHome retourne le chemin absolu du répertoire maison de
// l'utilisateur courant. Sert de point de départ au picker.
func (s *Server) handleFSHome(w http.ResponseWriter, r *http.Request) {
	home, err := os.UserHomeDir()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "NO_HOME", err.Error())
		return
	}
	writeJSON(w, map[string]string{"path": home})
}

// handleFSList énumère les sous-dossiers (et fichiers quand
// `files=1`) du path absolu passé en query. Files cachés et les
// grands dossiers de build / deps (.git, node_modules, etc.) sont
// masqués par défaut — peu utile pour un folder picker.
func (s *Server) handleFSList(w http.ResponseWriter, r *http.Request) {
	abs := r.URL.Query().Get("path")
	includeFiles := r.URL.Query().Get("files") == "1"

	if abs == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "NO_HOME", err.Error())
			return
		}
		abs = home
	}
	if !filepath.IsAbs(abs) {
		writeError(w, http.StatusBadRequest, "BAD_PATH", "chemin absolu requis")
		return
	}
	abs = filepath.Clean(abs)

	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "ce dossier n'existe pas")
			return
		}
		writeError(w, http.StatusInternalServerError, "STAT_FAILED", err.Error())
		return
	}
	if !info.IsDir() {
		writeError(w, http.StatusBadRequest, "NOT_A_DIR", "ce chemin n'est pas un dossier")
		return
	}

	raw, err := os.ReadDir(abs)
	if err != nil {
		writeError(w, http.StatusForbidden, "READ_FAILED", err.Error())
		return
	}

	// Filtres appliqués par défaut : cachés + gros dossiers qui
	// polluent le picker. L'opérateur peut toujours taper un chemin
	// manuel dans le champ si besoin.
	skip := map[string]bool{
		"node_modules": true,
		".git":         true,
		".svelte-kit":  true,
		".next":        true,
		".cache":       true,
		"dist":         true,
		"build":        true,
		"vendor":       true,
		"target":       true,
	}
	var entries []fsEntry
	for _, e := range raw {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if skip[name] {
			continue
		}
		if !e.IsDir() && !includeFiles {
			continue
		}
		entries = append(entries, fsEntry{
			Name:  name,
			Path:  filepath.Join(abs, name),
			IsDir: e.IsDir(),
		})
	}
	// Tri : dirs d'abord, puis alphabétique.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	parent := filepath.Dir(abs)
	if parent == abs {
		parent = ""
	}
	home, _ := os.UserHomeDir()
	writeJSON(w, fsListResponse{
		Path:    abs,
		Parent:  parent,
		Home:    home,
		Entries: entries,
	})
}

// handleFSMkdir crée un sous-dossier dans un parent absolu. Utilisé par
// le folder picker pour que l'opérateur puisse créer un dossier dédié
// à son projet sans quitter l'UI (ex. "je clique ~/Documents puis
// + Nouveau dossier > todolist → workdir = ~/Documents/todolist").
//
// Sécurité : le parent DOIT être un chemin absolu, le nom DOIT être
// un basename simple (pas de "../", pas de "/", pas de nom caché).
// On refuse de créer un dossier qui existerait déjà pour éviter
// qu'un clic accidentel réutilise un workdir existant avec du
// contenu perso.
func (s *Server) handleFSMkdir(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Parent string `json:"parent"`
		Name   string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<14)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	parent := strings.TrimSpace(body.Parent)
	name := strings.TrimSpace(body.Name)
	if parent == "" || name == "" {
		writeError(w, http.StatusBadRequest, "MISSING_FIELDS", "parent et name requis")
		return
	}
	if !filepath.IsAbs(parent) {
		writeError(w, http.StatusBadRequest, "BAD_PATH", "parent doit être absolu")
		return
	}
	// Refuse les noms qui sortent du parent (../) ou contiennent un /
	// ou démarrent par un . (dossiers cachés pas souhaités ici).
	if strings.ContainsAny(name, "/\\") || name == "." || name == ".." || strings.HasPrefix(name, ".") {
		writeError(w, http.StatusBadRequest, "BAD_NAME",
			"nom de dossier invalide (pas de / pas de .. pas de dossier caché)")
		return
	}
	// Longueur raisonnable.
	if len(name) > 128 {
		writeError(w, http.StatusBadRequest, "BAD_NAME", "nom trop long (max 128 caractères)")
		return
	}
	// Parent doit exister et être un dossier.
	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		writeError(w, http.StatusBadRequest, "BAD_PARENT", "parent introuvable ou n'est pas un dossier")
		return
	}
	full := filepath.Join(parent, name)
	if _, err := os.Stat(full); err == nil {
		writeError(w, http.StatusConflict, "ALREADY_EXISTS",
			"ce dossier existe déjà, choisis un autre nom")
		return
	}
	if err := os.Mkdir(full, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "MKDIR_FAILED", err.Error())
		return
	}
	writeJSON(w, map[string]string{"path": full})
}
