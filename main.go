package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jasonlvhit/gocron"
)

type File struct {
	FileName string
	UniqueID string
}

type FileDatabase struct {
	UniqueID   string `json:"uniqueID"`
	Path       string `json:"path"`
	Name       string `json:"name"`
	UploadTime string `json:"uploadTime"`
	Type       string `json:"type"`
}

type FileDatabaseJSON struct {
	Files []FileDatabase `json:"files"`
}

func dateFormat(layout string, d int64) string {
	intTime := int64(d)
	t := time.Unix(intTime, 0)
	if layout == "" {
		layout = "2006-01-02 15:04:05"
	}
	return t.Format(layout)
}

func generateRandomID(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var database []FileDatabase

	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomID := ""

	for i := 0; i < length; i++ {
		randomID += string(charset[random.Intn(len(charset))])
	}

	storage, err := os.Open("storage.json")

	if os.IsNotExist(err) {
		storage, err = os.Create("storage.json")

		if err != nil {
			fmt.Println(err)
			return ""
		}
	}

	json.NewDecoder(storage).Decode(&database)
	defer storage.Close()

	for _, file := range database {
		if file.UniqueID == randomID {
			return generateRandomID(length)
		}
	}

	return randomID
}

func deleteOldFiles() {
	var database map[string]FileDatabase

	file, err := os.Open("storage.json")

	if os.IsNotExist(err) {
		file, err = os.Create("storage.json")

		if err != nil {
			fmt.Println(err)
			return
		}
	}

	json.NewDecoder(file).Decode(&database)
	defer file.Close()

	change := false

	for _, file := range database {
		uploadTime, _ := strconv.ParseInt(file.UploadTime, 10, 64)
		if time.Now().Unix()-uploadTime > 86400*14 {
			change = true
			os.Remove(file.Path)
			delete(database, file.UniqueID)
		}
	}

	if !change {
		return
	}

	storage, err := os.OpenFile("storage.json", os.O_RDWR, 0644)

	if err != nil {
		fmt.Println(err)
		return
	}

	storage.Truncate(0)
	storage.Seek(0, 0)

	encoder := json.NewEncoder(storage)
	encoder.Encode(database)
	defer storage.Close()

	if err != nil {
		fmt.Println(err)
		return
	}
}

func executeEvery(function func(), seconds int) {
	gocron.Every(uint64(seconds)).Seconds().Do(function)
	<-gocron.Start()
}

func homePage(w http.ResponseWriter, r *http.Request) {
	template := template.Must(template.ParseFiles("templates/index.html"))
	template.Execute(w, nil)
}

func uploadFile(w http.ResponseWriter, r *http.Request) {
	file, fileHeader, err := r.FormFile("fileUpload")

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	defer file.Close()

	err = os.MkdirAll("storage", os.ModePerm)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	storage, err := os.OpenFile("storage.json", os.O_RDWR, 0644)

	if os.IsNotExist(err) {
		storage, err = os.Create("storage.json")

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	randomID := generateRandomID(10)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var fileDatabase map[string]FileDatabase

	json.NewDecoder(storage).Decode(&fileDatabase)

	fileDatabase[randomID] = FileDatabase{
		UniqueID:   randomID,
		Path:       fmt.Sprintf("storage/%s.%s", randomID, strings.Split(fileHeader.Filename, ".")[1]),
		Name:       fileHeader.Filename,
		UploadTime: strconv.FormatInt(time.Now().Unix(), 10),
		Type:       fileHeader.Header.Get("Content-Type"),
	}

	newFileBytes, err := json.Marshal(fileDatabase)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	storage.WriteAt(newFileBytes, 0)

	defer storage.Close()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	destinationFile, err := os.Create(fmt.Sprintf("storage/%s.%s", randomID, strings.Split(fileHeader.Filename, ".")[1]))

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, file)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	template := template.Must(template.ParseFiles("templates/upload.html"))
	fileHead := File{FileName: fileHeader.Filename, UniqueID: randomID}
	template.Execute(w, fileHead)
}

// TODO: Implement JSON or SQLite database to store file names and paths with a unique ID for reference and for the url path where to download also save the date and time of upload to delete after x days/hours

func showFilePage(w http.ResponseWriter, r *http.Request) {
	var database map[string]FileDatabase

	file, err := os.Open("storage.json")

	if os.IsNotExist(err) {
		file, err = os.Create("storage.json")

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	json.NewDecoder(file).Decode(&database)
	defer file.Close()

	fileURL := strings.Split(r.URL.Path, "/")
	fileUniqueID := fileURL[len(fileURL)-1]

	if _, ok := database[fileUniqueID]; !ok {
		http.Error(w, "File not found", http.StatusNotFound)

		// TODO: Redirect to 404 page
		return
	}

	fileInfo := database[fileUniqueID]
	uploadTime, err := strconv.ParseInt(fileInfo.UploadTime, 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fileInfo.UploadTime = dateFormat("", uploadTime)

	template := template.Must(template.ParseFiles("templates/download.html"))
	template.Execute(w, fileInfo)
}

func downloadFile(w http.ResponseWriter, r *http.Request) {
	var database map[string]FileDatabase

	file, err := os.Open("storage.json")

	if os.IsNotExist(err) {
		file, err = os.Create("storage.json")

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	json.NewDecoder(file).Decode(&database)
	defer file.Close()

	fileURL := strings.Split(r.URL.Path, "/")
	fileUniqueID := fileURL[len(fileURL)-1]

	if _, ok := database[fileUniqueID]; !ok {
		http.Error(w, "File not found", http.StatusNotFound)

		// TODO: Redirect to 404 page
		return
	}

	fileInfo := database[fileUniqueID]
	uploadTime, err := strconv.ParseInt(fileInfo.UploadTime, 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fileInfo.UploadTime = dateFormat("", uploadTime)

	file, err = os.Open(fileInfo.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", fileInfo.Type)
	w.Header().Set("Content-Disposition", "attachment; filename="+fileInfo.Name)

	http.ServeFile(w, r, fileInfo.Path)
}

func main() {
	deleteOldFiles()
	go executeEvery(deleteOldFiles, 60*60*12) // 12 hours
	fmt.Println("Starting server...")

	http.HandleFunc("/", homePage)
	http.HandleFunc("/upload", uploadFile)
	http.HandleFunc("/storage/", showFilePage)
	http.HandleFunc("/download/", downloadFile)
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".css") {
			w.Header().Set("Content-Type", "text/css")
			http.ServeFile(w, r, r.URL.Path[1:])
		}

		if strings.HasSuffix(r.URL.Path, ".png") {
			w.Header().Set("Content-Type", "image/png")
			http.ServeFile(w, r, r.URL.Path[1:])
		}
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
