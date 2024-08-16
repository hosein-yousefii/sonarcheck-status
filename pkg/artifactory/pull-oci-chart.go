package artifactory 

import (
	"log"
	"fmt"
        "net/http"
	"encoding/json"
        "strings"
        "os"
	"io"

	"sonarcheck/pkg/utils"
)

type Manifest struct {
        SchemaVersion int `json:"schemaVersion"`
        Config        struct {
                MediaType string `json:"mediaType"`
                Digest    string `json:"digest"`
                Size      int    `json:"size"`
        } `json:"config"`
        Layers []struct {
                MediaType string `json:"mediaType"`
                Digest    string `json:"digest"`
                Size      int    `json:"size"`
        } `json:"layers"`
}

// Check availability of jfrog artifactory
func CheckAvailability(jfrogURL string) error {
    resp, err := http.Get(jfrogURL)
    if err != nil {
        return fmt.Errorf("[ERROR] JFrog server is not reachable: %v", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("[ERROR] JFrog server returned non-OK status: %d", resp.StatusCode)
    }
    return nil
}

// Get oci helm chart from artifactory
func FetchOciChart(jfrogURL, ociRegistry, chartRepository, chartName, chartVersion, jfrogCredentials string, verbose, debug bool) {
	jfrogCredentialsParts := strings.SplitN(jfrogCredentials, ":", 2)

        // artifactory credentials
        user := jfrogCredentialsParts[0]
        password := jfrogCredentialsParts[1]

        // Construct the manifest URL
        manifestURL := fmt.Sprintf("%s/artifactory/%s/%s/%s/%s/manifest.json", jfrogURL, ociRegistry, chartRepository, chartName, chartVersion)

        // Make the HTTP GET request to fetch the manifest
        req, err := http.NewRequest("GET", manifestURL, nil)
        if err != nil {
                log.Fatalf("Error creating request:", err)
                return
        }
        req.SetBasicAuth(user, password)

        client := &http.Client{}
        resp, err := client.Do(req)
        if err != nil {
                log.Fatalf("Error making request:", err)
                return
        }
        defer resp.Body.Close()

        // Check if request was successful
        if resp.StatusCode != http.StatusOK {
                log.Fatalf("Error: failed to fetch manifest:", resp.Status)
                return
        }

        // Parse the JSON response
        var manifest Manifest
        err = json.NewDecoder(resp.Body).Decode(&manifest)
        if err != nil {
                log.Fatalf("Error decoding JSON:", err)
                return
        }

        // By having the manifest we are able to download the layers of the chart
        for i, layer := range manifest.Layers {
                sha256Digest := strings.TrimPrefix(layer.Digest, "sha256:")
                contentURL := fmt.Sprintf("%s/artifactory/%s/%s/%s/%s/sha256__%s", 
		jfrogURL, ociRegistry, chartRepository, chartName, chartVersion, sha256Digest)

                // Make the HTTP GET request to download the content
                req, err = http.NewRequest("GET", contentURL, nil)
                if err != nil {
                        log.Fatalf("Error creating request:", err)
                        return
                }
                req.SetBasicAuth(user, password)

                resp, err = client.Do(req)
                if err != nil {
                        log.Fatalf("Error making request:", err)
                        return
                }
                defer resp.Body.Close()
                // Check if request was successful
                if resp.StatusCode != http.StatusOK {
                        log.Printf("[Error]: failed to download content for layer %d: %s\n", i+1, resp.Status)
                        continue
                }

                // Create the output file with a unique name
                filename := fmt.Sprintf("layer_%d.tgz", i+1)
                outFile, err := os.Create(filename)
                if err != nil {
                        log.Fatalf("Error creating file:", err)
                        return
                }

                // Write the response body to the file
                _, err = io.Copy(outFile, resp.Body)
                outFile.Close() // Close the file after writing
                if err != nil {
                        log.Fatalf("Error writing file:", err)
                        return
                }

		if verbose || debug {
                      log.Printf("File %s downloaded successfully.\n", filename)
                }

                // Extract the .tgz chart
                err = utils.ExtractTarGz(filename, fmt.Sprintf("layer_%d", i+1))
                if err != nil {
                        log.Fatalf("Error extracting %s: %v\n", filename, err)
                } else if verbose || debug {
                        log.Printf("File %s extracted successfully.\n", filename)
                }
        }
}
