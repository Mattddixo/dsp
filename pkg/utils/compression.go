package utils

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
)

// Compress compresses data using zstd compression
func Compress(data []byte, level int) ([]byte, error) {
	// Create encoder with specified compression level
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(level)))
	if err != nil {
		return nil, fmt.Errorf("failed to create compressor: %w", err)
	}
	defer encoder.Close()

	// Compress data
	compressed := encoder.EncodeAll(data, nil)
	return compressed, nil
}

// Decompress decompresses data using zstd compression
func Decompress(data []byte) ([]byte, error) {
	// Create decoder
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create decompressor: %w", err)
	}
	defer decoder.Close()

	// Decompress data
	decompressed, err := decoder.DecodeAll(data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}

	return decompressed, nil
}

// HashBytes calculates SHA-256 hash of data
func HashBytes(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// CreateZipArchive creates a zip archive with the given files
func CreateZipArchive(zipPath string, files map[string]string) error {
	// Create zip file
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	// Create zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Add files to zip
	for name, path := range files {
		// Add empty file
		if path == "" {
			if _, err := zipWriter.Create(name); err != nil {
				return fmt.Errorf("failed to create zip entry: %w", err)
			}
			continue
		}

		// Add directory
		if path[len(path)-1] == '/' {
			// Create directory entry
			if _, err := zipWriter.Create(name); err != nil {
				return fmt.Errorf("failed to create directory entry: %w", err)
			}
			continue
		}

		// Add file
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		// Create parent directories in zip if needed
		if dir := filepath.Dir(name); dir != "." {
			if _, err := zipWriter.Create(dir + "/"); err != nil {
				return fmt.Errorf("failed to create directory entry: %w", err)
			}
		}

		writer, err := zipWriter.Create(name)
		if err != nil {
			return fmt.Errorf("failed to create zip entry: %w", err)
		}

		if _, err := io.Copy(writer, file); err != nil {
			return fmt.Errorf("failed to write file to zip: %w", err)
		}
	}

	return nil
}

// ExtractZipArchive extracts a zip archive to the given directory
func ExtractZipArchive(zipPath, destDir string) error {
	// Open zip file
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	// Extract each file
	for _, file := range reader.File {
		path := filepath.Join(destDir, file.Name)

		// Create directory if needed
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		// Open source file
		src, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open zip entry: %w", err)
		}

		// Create destination file
		dst, err := os.Create(path)
		if err != nil {
			src.Close()
			return fmt.Errorf("failed to create file: %w", err)
		}

		// Copy file contents
		if _, err := io.Copy(dst, src); err != nil {
			src.Close()
			dst.Close()
			return fmt.Errorf("failed to extract file: %w", err)
		}

		src.Close()
		dst.Close()
	}

	return nil
}

// UpdateZipFile updates a file in a zip archive
func UpdateZipFile(zipPath, filename string, data []byte) error {
	// Create temporary file
	tempFile, err := os.CreateTemp("", "dsp-zip-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Open zip file
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	// Create new zip writer
	writer := zip.NewWriter(tempFile)
	defer writer.Close()

	// Copy all files except the one to update
	for _, file := range reader.File {
		if file.Name == filename {
			continue
		}

		// Create new file entry
		dst, err := writer.Create(file.Name)
		if err != nil {
			return fmt.Errorf("failed to create zip entry: %w", err)
		}

		// Open source file
		src, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open zip entry: %w", err)
		}

		// Copy file contents
		if _, err := io.Copy(dst, src); err != nil {
			src.Close()
			return fmt.Errorf("failed to copy file: %w", err)
		}
		src.Close()
	}

	// Add updated file
	dst, err := writer.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create zip entry: %w", err)
	}

	if _, err := io.Copy(dst, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("failed to write updated file: %w", err)
	}

	// Close writer to flush changes
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close zip writer: %w", err)
	}

	// Replace original file with updated one
	if err := os.Rename(tempFile.Name(), zipPath); err != nil {
		return fmt.Errorf("failed to replace zip file: %w", err)
	}

	return nil
}
