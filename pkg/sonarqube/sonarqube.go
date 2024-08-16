package sonarqube

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "net/http"

    "sonarcheck/pkg/utils"
)

func CheckAvailability(sonarqubeURL, sonarqubeToken string) error {
    healthCheckURL := fmt.Sprintf("%s/api/server/version", sonarqubeURL)
    resp, err := http.Get(healthCheckURL)
    if err != nil {
        return fmt.Errorf("[ERROR] SonarQube server is not reachable: %v", err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("[ERROR] SonarQube server returned non-OK status: %d", resp.StatusCode)
    }
    return nil
}

// In order to find a project in Sonarqube, we need the projectKey
func FindProjectKey(dependency, sonarqubeURL, sonarqubeToken string) (string, error) {
    // Searching through the sonarqube to find the 
    searchURL := fmt.Sprintf("%s/api/components/search?qualifiers=TRK&q=%s", sonarqubeURL, dependency)
    client := &http.Client{}
    req, err := http.NewRequest("GET", searchURL, nil)
    if err != nil {
        return "", fmt.Errorf("[ERROR] creating GET request: %v", err)
    }
    req.SetBasicAuth(sonarqubeToken, "")
    resp, err := client.Do(req)
    if err != nil {
        return "", fmt.Errorf("[ERROR] sending GET request: %v", err)
    }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("[ERROR] reading response body: %v", err)
    }

    // Through the response make sure we get the key related to the specified project
    var responseMap map[string]interface{}
    if err := json.Unmarshal(body, &responseMap); err == nil {
        components, ok := responseMap["components"].([]interface{})
        if ok && len(components) > 0 {
            for _, comp := range components {
                component := comp.(map[string]interface{})
                if name, ok := component["name"].(string); ok && name == dependency {
                    if key, ok := component["key"].(string); ok {
                        return key, nil
                    }
                }
            }
        }
    }
    return "", fmt.Errorf("project key not found for dependency %s", dependency)
}

// Check the sonar status of a specific components version
func SonarCheck(dependency, version, sonarqubeURL, sonarqubeToken string, debug bool) (string, error) {
        // Construct the URL with the provided dependency key
        url := fmt.Sprintf("%s/api/project_analyses/search?ps=200&project=%s", sonarqubeURL, dependency)

        client := &http.Client{}
        req, err := http.NewRequest("GET", url, nil)
        if err != nil {
                return "", fmt.Errorf("[ERROR] creating GET request: %v", err)
        }

        req.SetBasicAuth(sonarqubeToken, "")
        resp, err := client.Do(req)
        if err != nil {
                return "", fmt.Errorf("[ERROR] sending GET request: %v", err)
        }
        defer resp.Body.Close()

        body, err := ioutil.ReadAll(resp.Body)
        if err != nil {
                return "", fmt.Errorf("[ERROR] reading response body: %v", err)
        }

        // Debug output
        if debug {
                responseString := string(body)
                fmt.Printf("Response for dependency %s-%s: %s\n", dependency, version, responseString)
        }

        // Parse the JSON response
        var responseMap map[string]interface{}
        if err := json.Unmarshal(body, &responseMap); err != nil {
                return "", fmt.Errorf("[ERROR] unmarshalling response: %v", err)
        }
        // Check for errors in the response
        if errors, ok := responseMap["errors"].([]interface{}); ok && len(errors) > 0 {
                firstError := errors[0].(map[string]interface{})
                if msg, ok := firstError["msg"].(string); ok {
                        return "", fmt.Errorf("SonarQube API error: %s", msg)
                }
        }

        // Find the analysis entry for the specified version or the nearest previous version
	// In Sonarqube projects history, when a project passes the quality gate continously, 
	// you will get the status from the first version that passed, and not the last one(last one doesn't have the status field).
	// so we need the check the lower versions status which has the field.
        analyses, ok := responseMap["analyses"].([]interface{})
        if !ok {
                return "", fmt.Errorf("unexpected response format: missing 'analyses'")
        }

        var latestQualityGateStatus string
        var foundVersion bool

        for _, analysis := range analyses {
                analysisMap, ok := analysis.(map[string]interface{})
                if !ok {
                        continue
                }

                projectVersion, ok := analysisMap["projectVersion"].(string)
                if !ok {
                        continue
                }

                // Skip versions greater than the specified version using numerical comparison
                versionComparison := utils.CompareVersions(projectVersion, version)
                if versionComparison == 0 {
                        foundVersion = true
                } else if foundVersion && versionComparison > 0 {
                        continue
                }
                // Check if this version has a QUALITY_GATE event
                events, ok := analysisMap["events"].([]interface{})
                if !ok {
                        continue
                }

                for _, event := range events {
                        eventMap, ok := event.(map[string]interface{})
                        if !ok {
                                continue
                        }
                        if category, ok := eventMap["category"].(string); ok && category == "QUALITY_GATE" {
                                if name, ok := eventMap["name"].(string); ok {
                                        latestQualityGateStatus = name
                                        return latestQualityGateStatus, nil
                                }
                        }
                }
        }
        if latestQualityGateStatus != "" {
                return latestQualityGateStatus, nil
        }
        return "No quality gate status found before the specified version", nil
}

