package config

import (
    "log"
    "os"
)

const (
    defaultSonarQubeURL = "http://sonarqube.com"
    defaultJfrogURL     = "http://artifactory.com"
    CleaningPattern = "layer_*"
    defaultOCIRegistry = "charts-registry"
    defaultChartRepository = "lieferscheine"
)

var (
    Verbose           bool
    Debug             bool
    SonarQubeURL      string
    JfrogURL          string
    SonarQubeToken    string
    JfrogCredentials  string
    OCIRegistry       string
    ChartRepository   string
    ChartName         string
    ChartVersion      string
)

func LoadEnv() {
    SonarQubeURL = getEnv("SONARQUBE_URL", defaultSonarQubeURL)
    JfrogURL = getEnv("JFROG_URL", defaultJfrogURL)
    OCIRegistry = getEnv("OCI_REGISTRY", defaultOCIRegistry)
    ChartRepository = getEnv("CHART_REPOSITORY", defaultChartRepository)
    SonarQubeToken = mustGetEnv("SONARQUBE_TOKEN")
    JfrogCredentials = mustGetEnv("JFROG_CREDENTIALS")
    ChartName = mustGetEnv("CHART_NAME")
    ChartVersion = mustGetEnv("CHART_VERSION")

    if os.Getenv("VERBOSE") == "true" {
        Verbose = true
    }
    if os.Getenv("DEBUG") == "true" {
        Debug = true
    }
}

func getEnv(key, defaultValue string) string {
    if value, exists := os.LookupEnv(key); exists {
        return value
    }
    return defaultValue
}

func mustGetEnv(key string) string {
    value, exists := os.LookupEnv(key)
    if !exists {
        log.Fatalf("[ERROR] %s environment variable not set", key)
    }
    return value
}
