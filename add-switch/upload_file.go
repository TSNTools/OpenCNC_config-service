package addswitch

//NNI

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

const uploadDir = "add-switch/uploads"

// Handler for serving the HTML upload form
func uploadForm(writer http.ResponseWriter, request *http.Request) {
	// Parse the HTML template
	tmpl, err := template.ParseFiles("add-switch/templates/upload.html")
	if err != nil {
		http.Error(writer, "Unable to load template", http.StatusInternalServerError)
		return
	}
	// Execute the template and serve the HTML
	tmpl.Execute(writer, nil)
}

// Handler for uploading and unzipping the file
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	// Limit the size of uploaded files (10 MB limit)
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Get the file from the form
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Unable to retrieve file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Check if the file is a .zip file
	if !isZipFile(file) {
		http.Error(w, "Uploaded file is not a valid .zip file", http.StatusBadRequest)
		return
	}

	// Unzip the file and extract only .yang files into memory
	extractedFiles, err := unzipFile(file)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error unzipping the file: %v", err), http.StatusInternalServerError)
		return
	}

	// Generate the JSON file name based on the .zip file name (without extension)
	baseName := strings.TrimSuffix(fileHeader.Filename, filepath.Ext(fileHeader.Filename))
	jsonFileName := fmt.Sprintf("%s.json", baseName)

	// Ensure the uploads directory exists before saving the JSON file
	err = os.MkdirAll(uploadDir, os.ModePerm)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create uploads directory: %v", err), http.StatusInternalServerError)
		return
	}

	// Save the extracted file names to a JSON file in memory (not disk)
	jsonFilePath := fmt.Sprintf("%s/%s", uploadDir, jsonFileName)
	err = saveToJSONFile(extractedFiles, jsonFilePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to save JSON file: %v", err), http.StatusInternalServerError)
		return
	}

	// Respond with the extracted files map as JSON
	w.Header().Set("Content-Type", "application/json")
	jsonResponse, err := json.Marshal(extractedFiles)
	if err != nil {
		http.Error(w, "Failed to marshal extracted files map", http.StatusInternalServerError)
		return
	}
	w.Write(jsonResponse)
}

// Helper function to check if the file is a .zip file
func isZipFile(file io.Reader) bool {
	// Check the first 4 bytes of the file to verify if it's a zip file (ZIP magic number: 0x50 0x4b 0x03 0x04)
	header := make([]byte, 4)
	_, err := file.Read(header)
	if err != nil {
		log.Println("Error reading file:", err)
		return false
	}
	// Check if the file starts with the zip magic number
	return string(header) == "PK\x03\x04"
}

// Helper function to unzip a .zip file and extract only .yang files
// Keeps the extracted files in memory
func unzipFile(file io.Reader) (map[string]bool, error) {
	// Initialize the map to store file names
	fileNamesMap := make(map[string]bool)

	// Create a buffer to read the file contents into memory
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file into buffer: %w", err)
	}

	// Create a zip.Reader from the buffer (which implements io.ReaderAt)
	zipReader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		return nil, fmt.Errorf("failed to create zip reader: %w", err)
	}

	// Iterate through each file in the zip archive and extract only .yang files
	for _, zipFile := range zipReader.File {
		// Check if the file has the .yang extension
		if !strings.HasSuffix(zipFile.Name, ".yang") {
			continue // Skip files that do not have a .yang extension
		}

		// Store the file name in the map (no need to save the actual file content)
		fileNamesMap[zipFile.Name] = true
	}

	// Return the map with file names
	return fileNamesMap, nil
}

// Helper function to save map to JSON file
func saveToJSONFile(data map[string]bool, filePath string) error {
	// Create or open the JSON file
	jsonFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create json file: %w", err)
	}
	defer jsonFile.Close()

	// Create an encoder and encode the data to the file
	encoder := json.NewEncoder(jsonFile)
	encoder.SetIndent("", "  ") // Pretty-print JSON with indentation
	err = encoder.Encode(data)
	if err != nil {
		return fmt.Errorf("failed to encode data to JSON: %w", err)
	}

	return nil
}

func Upload_File() {
	http.HandleFunc("/", uploadForm)
	http.HandleFunc("/upload", uploadHandler)

	fmt.Println("Server started at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
