package main

import (
	"io"
	"bytes"
	"fmt"
	"os"
	"net/http"
	"log"
	"path/filepath"
	"compress/gzip"
	"time"
	"encoding/json"
	"sync"
)

type FileMetadata struct {
	Filename string `json:"filename"`
	LastModifiedDate time.Time `json:"last_modified_date"`
	FileSizeGzipped int64 `json:"file_size_gzipped"`
	Files []FileMetadata `json:"files"`
}

type result struct {
	result FileMetadata
	error error
}

func gzipFile(file *os.File) (int64, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	defer gz.Close()

	if _, err := io.Copy(gz, file); err != nil {
		return 0, err
	}

	if err := gz.Close(); err != nil {
		return 0, err
	}

	return int64(buf.Len()), nil
}

func filepathToJSONMetadata(path string, resultChan chan result) {
	file, err := os.Open(path)
	if err != nil {
		resultChan <- result{FileMetadata{}, err}
		return
	}
	defer file.Close()

	fileInfo, err := os.Stat(path)
	if err != nil {
		resultChan <- result{FileMetadata{}, err}
		return
	}

	if fileInfo.IsDir() {
		files, err := os.ReadDir(path)
		if err != nil {
			resultChan <- result{FileMetadata{}, err}
			return
		}

		var wg = sync.WaitGroup{}
		c := make(chan result)

		for _, file := range files {
			wg.Add(1)
			go func(f os.DirEntry) {
				defer wg.Done()
				filepathToJSONMetadata(filepath.Join(path, file.Name()), c)
			}(file)
		}

		go func() {
			wg.Wait()
			close(c)
		}()

		subfiles := make([]FileMetadata, 0, len(files))
		for res := range c {
			if res.error != nil {
				resultChan <- result{FileMetadata{}, res.error}
				return
			}
			subfiles = append(subfiles, res.result)
		}

		resultChan <- result{
			FileMetadata{
				Filename: fileInfo.Name(),
				LastModifiedDate: fileInfo.ModTime(),
				FileSizeGzipped: 0,
				Files: subfiles,
			}, nil}
		return
	}

	gzippedSize, err := gzipFile(file)
	if err != nil {
		resultChan <- result{FileMetadata{}, err}
		return
	}

	resultChan <- result{
		FileMetadata{
			Filename: fileInfo.Name(),
			LastModifiedDate: fileInfo.ModTime(),
			FileSizeGzipped: gzippedSize,
		}, nil}
}

func fileMetadataHandler(w http.ResponseWriter, r *http.Request) {
	dir, err := os.Getwd()
	if err != nil {
		http.Error(w, "Error getting working directory", http.StatusInternalServerError)
		return
	}

	path := filepath.Join(dir, r.URL.Path)

	// create a channel to receive the results on
	c := make(chan result)
	go filepathToJSONMetadata(path, c)
	res := <-c

	if err := res.error; err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		fmt.Println(res.error)
		http.Error(w, "Error reading file ", http.StatusInternalServerError)
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(res.result); err != nil {
		http.Error(w, "Error generating JSON", http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
}

func main() {
	http.HandleFunc("/", fileMetadataHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
