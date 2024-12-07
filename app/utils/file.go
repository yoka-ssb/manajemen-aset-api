package utils

import (
	"io"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

var _ = godotenv.Load(".env")
var NC_API_ENDPOINT = os.Getenv("NEXTCLOUD_API_ENDPOINT")
var NC_ASSET_PATH = os.Getenv("NEXTCLOUD_ASSET_PATH")
var NC_USERNAME = os.Getenv("NEXTCLOUD_USERNAME")
var NC_PASSWORD = os.Getenv("NEXTCLOUD_PASSWORD")

func UploadFile(w http.ResponseWriter, r *http.Request, module string) (filePath *string, err error) {

	// Parse the multipart form
	err = r.ParseMultipartForm(10 << 20) // Max memory 10MB
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Default().Println(err)
		return nil, err
	}

	// Get the file from the request
	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Default().Println(err)
		return nil, err
	}
	defer file.Close()

	// Create a new HTTP client
	client := &http.Client{}

	// Set the API endpoint and credentials
	apiEndpoint := NC_API_ENDPOINT + NC_ASSET_PATH + module + "/" + handler.Filename
	req, err := http.NewRequest("PUT", apiEndpoint, file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Default().Println(err)
		return nil, err
	}
	req.SetBasicAuth(NC_USERNAME, NC_PASSWORD)
	req.Header.Set("Content-Type", handler.Header.Get("Content-Type"))

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Default().Println(err)
		return nil, err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusCreated {
		http.Error(w, resp.Status, resp.StatusCode)
		log.Default().Println(err)
		return nil, err
	}

	path := module + "/" + handler.Filename

	return &path, nil
}

func GetFile(w http.ResponseWriter, r *http.Request, filePath string) ([]byte, error) {

	// Create a new HTTP client
	client := &http.Client{}

	// Set the API endpoint and credentials
	apiEndpoint := NC_API_ENDPOINT + NC_ASSET_PATH + filePath
	req, err := http.NewRequest("GET", apiEndpoint, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Default().Println(err)
		return nil, err
	}
	req.SetBasicAuth(NC_USERNAME, NC_PASSWORD)

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Default().Println(err)
		return nil, err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		http.Error(w, resp.Status, resp.StatusCode)
		log.Default().Println(err)
		return nil, err
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Error reading response body", http.StatusInternalServerError)
		log.Default().Println("Error reading response body:", err)
		return nil, err
	}

	// Write the response body to the client
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(http.StatusOK)
	_, writeErr := w.Write(body)
	if writeErr != nil {
		log.Default().Println("Error writing response:", writeErr)
		return nil, writeErr
	}

	return body, nil
}