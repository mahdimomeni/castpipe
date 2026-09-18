package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestResolveUniquePath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "castpipe_test_resolve_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// First path should be original filename
	p1 := ResolveUniquePath(tempDir, "sample.txt")
	if filepath.Base(p1) != "sample.txt" {
		t.Errorf("Expected sample.txt, got %s", filepath.Base(p1))
	}

	// Create sample.txt
	if err := os.WriteFile(p1, []byte("first"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Second path should be sample (1).txt
	p2 := ResolveUniquePath(tempDir, "sample.txt")
	if filepath.Base(p2) != "sample (1).txt" {
		t.Errorf("Expected sample (1).txt, got %s", filepath.Base(p2))
	}

	// Create sample (1).txt
	if err := os.WriteFile(p2, []byte("second"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Third path should be sample (2).txt
	p3 := ResolveUniquePath(tempDir, "sample.txt")
	if filepath.Base(p3) != "sample (2).txt" {
		t.Errorf("Expected sample (2).txt, got %s", filepath.Base(p3))
	}
}

func TestServerEndpoints(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "castpipe_test_server_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	srv, err := NewServer(tempDir)
	if err != nil {
		t.Fatalf("Failed to initialize server: %v", err)
	}
	srv.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	}()

	if srv.Port() <= 0 {
		t.Errorf("Expected positive port, got %d", srv.Port())
	}
	if srv.IP() == "" {
		t.Error("Expected detected IP, got empty string")
	}

	baseURL := "http://127.0.0.1:" + strings.TrimSpace(string(rune(srv.Port())))
	baseURL = "http://127.0.0.1:" + filepath.Clean(tempDir) // replace with actual port
	baseURL = "http://" + netJoinHostPort("127.0.0.1", srv.Port())

	// Test Health
	resp, err := http.Get(baseURL + "/api/health")
	if err != nil {
		t.Fatalf("Health request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Health returned status %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Test Drop Snippet
	var receivedDrop DropItem
	var wg sync.WaitGroup
	wg.Add(1)

	srv.SetDropCallback(func(item DropItem) {
		receivedDrop = item
		wg.Done()
	})

	snippetReq := DropSnippetPayload{
		Sender:  "TestNode",
		Syntax:  "yaml",
		Content: "test: true\nversion: 1.0",
	}
	snippetBody, _ := json.Marshal(snippetReq)

	dropResp, err := http.Post(baseURL+"/api/drop", "application/json", bytes.NewReader(snippetBody))
	if err != nil {
		t.Fatalf("Drop request failed: %v", err)
	}
	if dropResp.StatusCode != http.StatusOK {
		t.Errorf("Drop returned status %d", dropResp.StatusCode)
	}
	dropResp.Body.Close()

	wg.Wait()

	if receivedDrop.Type != DropTypeSnippet {
		t.Errorf("Expected DropTypeSnippet, got %s", receivedDrop.Type)
	}
	if receivedDrop.Snippet == nil || receivedDrop.Snippet.Content != snippetReq.Content {
		t.Errorf("Snippet content mismatch")
	}
	if receivedDrop.Snippet.Syntax != "yaml" {
		t.Errorf("Expected syntax yaml, got %s", receivedDrop.Snippet.Syntax)
	}

	// Test Upload File
	wg.Add(1)
	var receivedFile DropItem
	srv.SetDropCallback(func(item DropItem) {
		receivedFile = item
		wg.Done()
	})

	bodyBuf := &bytes.Buffer{}
	mpWriter := multipart.NewWriter(bodyBuf)
	filePart, err := mpWriter.CreateFormFile("files", "script.sh")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	testContent := "#!/usr/bin/env bash\necho 'hello castpipe'\n"
	if _, err := io.WriteString(filePart, testContent); err != nil {
		t.Fatalf("Failed writing content to form: %v", err)
	}
	mpWriter.Close()

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/upload?sender=UploadTester", bodyBuf)
	if err != nil {
		t.Fatalf("Failed to create upload request: %v", err)
	}
	req.Header.Set("Content-Type", mpWriter.FormDataContentType())

	uploadResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Upload request failed: %v", err)
	}
	if uploadResp.StatusCode != http.StatusOK {
		t.Errorf("Upload returned status %d", uploadResp.StatusCode)
	}
	uploadResp.Body.Close()

	wg.Wait()

	if receivedFile.Type != DropTypeFile {
		t.Errorf("Expected DropTypeFile, got %s", receivedFile.Type)
	}
	if receivedFile.File == nil {
		t.Fatal("Expected File payload to not be nil")
	}
	if receivedFile.File.FileName != "script.sh" {
		t.Errorf("Expected script.sh, got %s", receivedFile.File.FileName)
	}
	if receivedFile.File.FileSize != int64(len(testContent)) {
		t.Errorf("Expected file size %d, got %d", len(testContent), receivedFile.File.FileSize)
	}

	savedBytes, err := os.ReadFile(receivedFile.File.FilePath)
	if err != nil {
		t.Fatalf("Failed to read saved file from disk: %v", err)
	}
	if string(savedBytes) != testContent {
		t.Errorf("Saved content mismatch: expected %s, got %s", testContent, string(savedBytes))
	}
}

func netJoinHostPort(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}

func TestNewServerWithConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "castpipe_test_cfg_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Test custom hostname and download dir
	cfg := ServerConfig{
		DownloadDir: filepath.Join(tempDir, "CustomDrops"),
		Hostname:    "Dev-Terminal-Alpha",
		Port:        0, // ephemeral
	}

	srv, err := NewServerWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewServerWithConfig failed: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	}()

	if srv.Hostname() != "Dev-Terminal-Alpha" {
		t.Errorf("Expected hostname Dev-Terminal-Alpha, got %s", srv.Hostname())
	}
	if srv.DownloadDir() != filepath.Join(tempDir, "CustomDrops") {
		t.Errorf("Expected download dir %s, got %s", filepath.Join(tempDir, "CustomDrops"), srv.DownloadDir())
	}
	if srv.Port() <= 0 {
		t.Errorf("Expected positive ephemeral port, got %d", srv.Port())
	}

	// 2. Test specific port allocation
	specificPort := srv.Port() // already bound
	_, err = NewServerWithConfig(ServerConfig{Port: specificPort})
	if err == nil {
		t.Error("Expected error when binding already-in-use port")
	}
}

