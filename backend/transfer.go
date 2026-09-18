package backend

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

var defaultHTTPClient = &http.Client{
	Timeout: 30 * time.Minute, // Long timeout for large streaming uploads
}

// SendSnippet sends a code snippet or terminal command to a remote peer.
func SendSnippet(senderName, targetIP string, targetPort int, content, syntax string) error {
	if targetIP == "" || targetPort <= 0 {
		return errors.New("invalid target IP or port")
	}

	payload := DropSnippetPayload{
		ID:        uuid.NewString(),
		Sender:    senderName,
		Syntax:    syntax,
		Content:   content,
		Timestamp: time.Now().UnixMilli(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal snippet payload: %w", err)
	}

	url := fmt.Sprintf("http://%s:%d/api/drop", targetIP, targetPort)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to build snippet request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send snippet to %s:%d: %w", targetIP, targetPort, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remote node returned error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// SendPath streams a single regular file or zips and streams a folder on-the-fly without disk buffers.
func SendPath(senderName, targetIP string, targetPort int, path string) error {
	cleanPath := filepath.Clean(filepath.FromSlash(path))
	stat, err := os.Stat(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to inspect path %s: %w", cleanPath, err)
	}

	uploadURL := fmt.Sprintf("http://%s:%d/api/upload", targetIP, targetPort)

	if stat.IsDir() {
		return streamDirectoryZip(senderName, uploadURL, cleanPath)
	}

	return streamRegularFile(senderName, uploadURL, cleanPath)
}

// SendPaths streams multiple files or directories sequentially.
func SendPaths(senderName, targetIP string, targetPort int, paths []string) error {
	var errs []string
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		if err := SendPath(senderName, targetIP, targetPort, p); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", filepath.Base(p), err))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// streamRegularFile streams a file directly into a multipart HTTP POST request without reading it into memory.
func streamRegularFile(senderName, uploadURL, filePath string) error {
	pr, pw := io.Pipe()
	mpWriter := multipart.NewWriter(pw)
	contentType := mpWriter.FormDataContentType()

	go func() {
		var closeErr error
		defer func() {
			if closeErr != nil && !errors.Is(closeErr, io.ErrClosedPipe) {
				_ = pw.CloseWithError(closeErr)
			} else {
				_ = pw.Close()
			}
		}()

		file, err := os.Open(filePath)
		if err != nil {
			closeErr = fmt.Errorf("failed to open source file: %w", err)
			return
		}
		defer file.Close()

		filename := filepath.Base(filePath)
		part, err := mpWriter.CreateFormFile("files", filename)
		if err != nil {
			closeErr = fmt.Errorf("failed to create multipart form part: %w", err)
			return
		}

		if _, err := io.Copy(part, file); err != nil {
			closeErr = fmt.Errorf("streaming error during file upload: %w", err)
			return
		}

		if err := mpWriter.Close(); err != nil {
			closeErr = fmt.Errorf("failed to finalize multipart writer: %w", err)
			return
		}
	}()

	req, err := http.NewRequest(http.MethodPost, uploadURL, pr)
	if err != nil {
		_ = pr.Close()
		return fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	if senderName != "" {
		req.Header.Set("X-Sender", senderName)
	}

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		_ = pr.Close()
		return fmt.Errorf("upload transfer failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server rejected upload with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// streamDirectoryZip zips a folder on-the-fly using io.Pipe() and streams directly into multipart upload without temporary files.
func streamDirectoryZip(senderName, uploadURL, dirPath string) error {
	pr, pw := io.Pipe()
	mpWriter := multipart.NewWriter(pw)
	contentType := mpWriter.FormDataContentType()
	archiveName := filepath.Base(dirPath) + ".zip"

	go func() {
		var closeErr error
		defer func() {
			if closeErr != nil && !errors.Is(closeErr, io.ErrClosedPipe) {
				_ = pw.CloseWithError(closeErr)
			} else {
				_ = pw.Close()
			}
		}()

		part, err := mpWriter.CreateFormFile("files", archiveName)
		if err != nil {
			closeErr = fmt.Errorf("failed to create form file for zip: %w", err)
			return
		}

		zipWriter := zip.NewWriter(part)

		walkErr := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			relPath, err := filepath.Rel(dirPath, path)
			if err != nil {
				return err
			}
			if relPath == "." {
				return nil
			}

			// Normalizing zip entry path with forward slashes as per ZIP standard
			zipEntryName := filepath.ToSlash(relPath)
			if info.IsDir() {
				zipEntryName += "/"
			}

			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			header.Name = zipEntryName
			header.Method = zip.Deflate

			entryWriter, err := zipWriter.CreateHeader(header)
			if err != nil {
				return err
			}

			if !info.IsDir() {
				srcFile, err := os.Open(path)
				if err != nil {
					return err
				}
				_, err = io.Copy(entryWriter, srcFile)
				_ = srcFile.Close()
				if err != nil {
					return err
				}
			}
			return nil
		})

		if walkErr != nil {
			closeErr = fmt.Errorf("directory traversal failed: %w", walkErr)
			return
		}

		if err := zipWriter.Close(); err != nil {
			closeErr = fmt.Errorf("failed to finalize zip stream: %w", err)
			return
		}

		if err := mpWriter.Close(); err != nil {
			closeErr = fmt.Errorf("failed to finalize multipart stream: %w", err)
			return
		}
	}()

	req, err := http.NewRequest(http.MethodPost, uploadURL, pr)
	if err != nil {
		_ = pr.Close()
		return fmt.Errorf("failed to build zip upload request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	if senderName != "" {
		req.Header.Set("X-Sender", senderName)
	}

	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		_ = pr.Close()
		return fmt.Errorf("zip transfer failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server rejected zip upload with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// OpenFolderInExplorer opens the specified folder path in Windows Explorer.
func OpenFolderInExplorer(folderPath string) error {
	cleanPath := filepath.Clean(filepath.FromSlash(folderPath))
	cmd := exec.Command("cmd.exe", "/c", "start", "", cleanPath)
	return cmd.Start()
}
