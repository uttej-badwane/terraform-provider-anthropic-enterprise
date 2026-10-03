package mock

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"slices"
	"strings"
	"unicode"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

// maxMemoryBytes is the API's limit on one memory's content.
const maxMemoryBytes = 102400

// validMemoryPath applies the reference's path rules: a leading "/", at least
// one segment, no empty, "." or ".." segments, no control or format
// characters, at most 1,024 bytes.
func validMemoryPath(p string) bool {
	if len(p) < 2 || len(p) > 1024 || !strings.HasPrefix(p, "/") {
		return false
	}
	for _, seg := range strings.Split(p[1:], "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	for _, r := range p {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) || r == ' ' || r == ' ' {
			return false
		}
	}
	return true
}

func contentHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// memoryView renders a memory for a response. The basic view omits content;
// the hash and size stay populated.
func memoryView(m *client.Memory, full bool) client.Memory {
	out := *m
	if !full {
		out.Content = nil
	}
	return out
}

// wantFull reports whether the response should carry content, given the
// endpoint's default.
func wantFull(r *http.Request, def bool) bool {
	switch r.URL.Query().Get("view") {
	case "full":
		return true
	case "basic":
		return false
	}
	return def
}

func (s *Server) findMemory(storeID, id string) *client.Memory {
	for _, m := range s.store.agentsStore.memories {
		if m.MemoryStoreID == storeID && m.ID == id {
			return m
		}
	}
	return nil
}

func (s *Server) memoryPathTaken(storeID, p, except string) bool {
	for _, m := range s.store.agentsStore.memories {
		if m.MemoryStoreID == storeID && m.Path == p && m.ID != except {
			return true
		}
	}
	return false
}

// memoryStoreFor resolves the store in the path and writes a 404 when it is
// missing.
func (s *Server) memoryStoreFor(w http.ResponseWriter, r *http.Request) (string, bool) {
	if !s.requireAgents(w, r, familyMemory) {
		return "", false
	}
	id := r.PathValue("id")
	if s.findMemoryStore(id) == nil {
		notFound(w, "memory_store")
		return "", false
	}
	return id, true
}

func (s *Server) listMemories(w http.ResponseWriter, r *http.Request) {
	storeID, ok := s.memoryStoreFor(w, r)
	if !ok {
		return
	}
	prefix := r.URL.Query().Get("path_prefix")
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		writeError(w, 400, "invalid_request_error", "path_prefix must end with /")
		return
	}
	full := wantFull(r, false)
	out := []client.Memory{}
	for _, m := range s.store.agentsStore.memories {
		if m.MemoryStoreID == storeID && strings.HasPrefix(m.Path, prefix) {
			out = append(out, memoryView(m, full))
		}
	}
	tokenList(w, r, out)
}

func (s *Server) createMemory(w http.ResponseWriter, r *http.Request) {
	storeID, ok := s.memoryStoreFor(w, r)
	if !ok {
		return
	}
	var in struct {
		Path    string  `json:"path"`
		Content *string `json:"content"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Content == nil {
		writeError(w, 400, "invalid_request_error", "content is required; pass \"\" for an empty memory")
		return
	}
	if !validMemoryPath(in.Path) {
		writeError(w, 400, "invalid_request_error", "invalid path")
		return
	}
	if len(*in.Content) > maxMemoryBytes {
		writeError(w, 400, "invalid_request_error", "content exceeds 102400 bytes")
		return
	}
	if s.memoryPathTaken(storeID, in.Path, "") {
		writeError(w, http.StatusConflict, "conflict_error", "a memory already exists at "+in.Path)
		return
	}
	content := *in.Content
	m := &client.Memory{
		ID: s.nextID("mem"), Type: "memory", MemoryStoreID: storeID, MemoryVersionID: s.nextID("memver"),
		Path: in.Path, Content: &content, ContentSHA256: contentHash(content), ContentSizeBytes: int64(len(content)),
		CreatedAt: now(), UpdatedAt: now(),
	}
	s.store.agentsStore.memories = append(s.store.agentsStore.memories, m)
	writeJSON(w, memoryView(m, wantFull(r, false)))
}

func (s *Server) getMemory(w http.ResponseWriter, r *http.Request) {
	storeID, ok := s.memoryStoreFor(w, r)
	if !ok {
		return
	}
	m := s.findMemory(storeID, r.PathValue("mem"))
	if m == nil {
		notFound(w, "memory")
		return
	}
	writeJSON(w, memoryView(m, wantFull(r, true)))
}

func (s *Server) updateMemory(w http.ResponseWriter, r *http.Request) {
	storeID, ok := s.memoryStoreFor(w, r)
	if !ok {
		return
	}
	m := s.findMemory(storeID, r.PathValue("mem"))
	if m == nil {
		notFound(w, "memory")
		return
	}
	var in client.MemoryUpdate
	if !decode(w, r, &in) {
		return
	}
	if p := in.Precondition; p != nil && p.ContentSHA256 != m.ContentSHA256 {
		writeError(w, http.StatusConflict, "memory_precondition_failed_error", "content_sha256 does not match the stored memory")
		return
	}
	if in.Path != nil {
		if !validMemoryPath(*in.Path) {
			writeError(w, 400, "invalid_request_error", "invalid path")
			return
		}
		if s.memoryPathTaken(storeID, *in.Path, m.ID) {
			writeError(w, http.StatusConflict, "conflict_error", "a memory already exists at "+*in.Path)
			return
		}
	}
	if in.Content != nil && len(*in.Content) > maxMemoryBytes {
		writeError(w, 400, "invalid_request_error", "content exceeds 102400 bytes")
		return
	}
	if in.Path != nil {
		m.Path = *in.Path
	}
	if in.Content != nil {
		content := *in.Content
		m.Content, m.ContentSHA256, m.ContentSizeBytes = &content, contentHash(content), int64(len(content))
		m.MemoryVersionID = s.nextID("memver")
	}
	m.UpdatedAt = now()
	writeJSON(w, memoryView(m, wantFull(r, false)))
}

func (s *Server) deleteMemory(w http.ResponseWriter, r *http.Request) {
	storeID, ok := s.memoryStoreFor(w, r)
	if !ok {
		return
	}
	id := r.PathValue("mem")
	i := slices.IndexFunc(s.store.agentsStore.memories, func(m *client.Memory) bool { return m.MemoryStoreID == storeID && m.ID == id })
	if i < 0 {
		notFound(w, "memory")
		return
	}
	s.store.agentsStore.memories = slices.Delete(s.store.agentsStore.memories, i, i+1)
	deleted(w, id, "memory_deleted")
}

// RewriteMemory changes a memory's content as an agent would, for drift tests.
func (s *Server) RewriteMemory(storeID, id, content string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.findMemory(storeID, id)
	if m == nil {
		return false
	}
	m.Content, m.ContentSHA256, m.ContentSizeBytes = &content, contentHash(content), int64(len(content))
	m.MemoryVersionID = s.nextID("memver")
	return true
}
