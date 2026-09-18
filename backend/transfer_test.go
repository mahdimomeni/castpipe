package backend

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestSendSnippet(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "castpipe_test_transfer_snippet_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	srv, err := NewServer(tempDir)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	srv.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	}()

	var received DropItem
	var wg sync.WaitGroup
	wg.Add(1)

	srv.SetDropCallback(func(item DropItem) {
		received = item
		wg.Done()
	})

	err = SendSnippet("SenderMachine", "127.0.0.1", srv.Port(), "echo 'test command'", "bash")
	if err != nil {
		t.Fatalf("SendSnippet failed: %v", err)
	}

	wg.Wait()

	if received.Type != DropTypeSnippet {
		t.Fatalf("Expected DropTypeSnippet, got %s", received.Type)
	}
	if received.Snippet.Content != "echo 'test command'" {
		t.Errorf("Unexpected content: %s", received.Snippet.Content)
	}
	if received.Snippet.Syntax != "bash" {
		t.Errorf("Unexpected syntax: %s", received.Snippet.Syntax)
	}
}

func TestSendPathRegularFile(t *testing.T) {
	destDir, err := os.MkdirTemp("", "castpipe_test_dest_*")
	if err != nil {
		t.Fatalf("Failed to create dest dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	srcDir, err := os.MkdirTemp("", "castpipe_test_src_*")
	if err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}
	defer os.RemoveAll(srcDir)

	srcFile := filepath.Join(srcDir, "config.json")
	content := []byte(`{"env":"production","port":8080}`)
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("Failed to write src file: %v", err)
	}

	srv, err := NewServer(destDir)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	srv.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	}()

	var received DropItem
	var wg sync.WaitGroup
	wg.Add(1)

	srv.SetDropCallback(func(item DropItem) {
		received = item
		wg.Done()
	})

	err = SendPath("SenderNode", "127.0.0.1", srv.Port(), srcFile)
	if err != nil {
		t.Fatalf("SendPath failed: %v", err)
	}

	wg.Wait()

	if received.Type != DropTypeFile {
		t.Fatalf("Expected DropTypeFile, got %s", received.Type)
	}
	if received.File.FileName != "config.json" {
		t.Errorf("Expected config.json, got %s", received.File.FileName)
	}

	savedBytes, err := os.ReadFile(received.File.FilePath)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}
	if string(savedBytes) != string(content) {
		t.Errorf("Saved content mismatch: got %s, expected %s", string(savedBytes), string(content))
	}
}

func TestSendPathDirectoryStreamingZip(t *testing.T) {
	destDir, err := os.MkdirTemp("", "castpipe_dest_zip_*")
	if err != nil {
		t.Fatalf("Failed to create dest dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	// Create test folder with nested hierarchy
	srcFolder, err := os.MkdirTemp("", "castpipe_project_*")
	if err != nil {
		t.Fatalf("Failed to create src folder: %v", err)
	}
	defer os.RemoveAll(srcFolder)

	subDir := filepath.Join(srcFolder, "submodule")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subDir: %v", err)
	}

	file1 := filepath.Join(srcFolder, "root.txt")
	file2 := filepath.Join(subDir, "nested.go")
	if err := os.WriteFile(file1, []byte("root file content"), 0644); err != nil {
		t.Fatalf("Failed to write root file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("package submodule\nfunc Init(){}"), 0644); err != nil {
		t.Fatalf("Failed to write nested file: %v", err)
	}

	srv, err := NewServer(destDir)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	srv.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	}()

	var received DropItem
	var wg sync.WaitGroup
	wg.Add(1)

	srv.SetDropCallback(func(item DropItem) {
		received = item
		wg.Done()
	})

	err = SendPath("ArchiveSender", "127.0.0.1", srv.Port(), srcFolder)
	if err != nil {
		t.Fatalf("SendPath directory failed: %v", err)
	}

	wg.Wait()

	if received.Type != DropTypeFile {
		t.Fatalf("Expected DropTypeFile, got %s", received.Type)
	}
	if !received.File.IsArchive {
		t.Error("Expected IsArchive to be true for directory drop")
	}

	// Verify the saved zip archive can be extracted and contains all files
	zipReader, err := zip.OpenReader(received.File.FilePath)
	if err != nil {
		t.Fatalf("Received file is not a valid zip archive: %v", err)
	}
	defer zipReader.Close()

	foundRoot := false
	foundNested := false

	for _, f := range zipReader.File {
		if f.Name == "root.txt" {
			foundRoot = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("Failed to open zip file entry: %v", err)
			}
			bytes, _ := io.ReadAll(rc)
			rc.Close()
			if string(bytes) != "root file content" {
				t.Errorf("Unexpected root.txt content: %s", string(bytes))
			}
		} else if f.Name == "submodule/nested.go" {
			foundNested = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("Failed to open zip file entry: %v", err)
			}
			bytes, _ := io.ReadAll(rc)
			rc.Close()
			if string(bytes) != "package submodule\nfunc Init(){}" {
				t.Errorf("Unexpected nested.go content: %s", string(bytes))
			}
		}
	}

	if !foundRoot {
		t.Error("root.txt was not found in streamed zip archive")
	}
	if !foundNested {
		t.Error("submodule/nested.go was not found in streamed zip archive")
	}
}
