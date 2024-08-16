package jfrog

import (
    "fmt"
    "net/http"
    "encoding/json"
    "strings"
    "io/ioutil"
)

// define Artifactory response type
type ArtifactoryResponse struct {
        Results []struct {
                URI string `json:"uri"`
        } `json:"results"`
        Errors []struct {
                Status  int    `json:"status"`
                Message string `json:"message"`
        } `json:"errors"`
}

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

func FindDependencyVersion(dependency, sha256, jfrogURL, jfrogCredentials string) (string, error) {
        searchURL := fmt.Sprintf("%sartifactory/api/search/checksum?sha256=%s", jfrogURL, sha256)

        client := &http.Client{}
        req, err := http.NewRequest("GET", searchURL, nil)
        if err != nil {
                return "", fmt.Errorf("creating GET request: %v", err)
        }

        // Split credentials into user and password
        parts := strings.SplitN(jfrogCredentials, ":", 2)
        if len(parts) != 2 {
                return "", fmt.Errorf("[ERROR] JFROG_CREDENTIALS environment variable is not properly formatted, e.x. user:pass")
        }
        user := parts[0]
        password := parts[1]

        req.SetBasicAuth(user, password)
        resp, err := client.Do(req)
        if err != nil {
                return "", fmt.Errorf("sending GET request: %v", err)
        }
        defer resp.Body.Close()

        body, err := ioutil.ReadAll(resp.Body)
        if err != nil {
                return "", fmt.Errorf("reading response body: %v", err)
        }

        var artifactoryResponse ArtifactoryResponse
        if err := json.Unmarshal(body, &artifactoryResponse); err != nil {
                return "", fmt.Errorf("unmarshalling JSON response: %v", err)
        }

        // Check for errors in the response
        if len(artifactoryResponse.Errors) > 0 {
                firstError := artifactoryResponse.Errors[0]
                return "", fmt.Errorf("API error: %s (status %d)", firstError.Message, firstError.Status)
        }
        // Check if the results array is empty
        if len(artifactoryResponse.Results) == 0 {
                return "", fmt.Errorf("no results found for sha256: %s", sha256)
        }

        // Extract the version from the URI
        uri := artifactoryResponse.Results[0].URI
        parts = strings.Split(uri, "/")
        if len(parts) < 2 {
                return "", fmt.Errorf("unable to extract version from URI: %s", uri)
        }

        // Assuming the version is the second last part of the URI
        version := parts[len(parts)-2]
        return version, nil
}

