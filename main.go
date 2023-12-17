package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
)

type File struct {
	FileName string
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

	destinationFile, err := os.Create(fmt.Sprintf("storage/%s", fileHeader.Filename))

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
	fileHead := File{FileName: fileHeader.Filename}
	template.Execute(w, fileHead)
}

func main() {
	fmt.Print("Starting server...")

	http.HandleFunc("/", homePage)
	http.HandleFunc("/upload", uploadFile)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
