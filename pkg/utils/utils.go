package utils

import (
    "fmt"
    "log"
    "regexp"
    "os"
    "strings"
    "strconv"
    "path/filepath"
    "archive/tar"
    "compress/gzip"
    "io"
    "io/ioutil"

    "gopkg.in/yaml.v2"
)

func Help() {
        fmt.Println(`Usage: SonarCheck [OPTIONS] [ignore rules]

Check umbrella charts if their subcharts (dependencies) are already passed the Sonarqube analyses.

Options:
  -h                Show this help message and exit
  -v                Enable verbose mode (This will override VERBOSE env)
  -d                Enable debug mode (This will override DEBUG env)

Environments:
  SONARQUBE_TOKEN   Required  Token that you need to check the status of applications
  JFROG_CREDENTIALS Required  Credentials that you need to get the version of dependencies. e.x. user:password
  CHART_NAME        Required  Umbrella chart name e.x. tramon-lt
  CHART_VERSION     Required  Umbrella chart version e.x. 202406.827.0
  SONARQUBE_URL     Optional  The URL of the sonarqube instance (default: http://sonar.com/)
  JFROG_URL         Optional  The URL of the jfrog instance (default: http://artifactory.com/)
  VERBOSE           Optional  Enable verbose mode
  DEBUG             Optional  Enable debug mode

Ignore rules:
  You can specify comma separated ignore patterns to not to check sonarqube status for those which match e.x.

  patterns=".*lt-textzeilen.*,.*-mock"
  SonarCheck -v "$buildinfo" "$patterns"

  If it matches, you will see the similar output by passing -v
  2024/08/08 11:31:26 web-app-1-mock: Ignored

  Command example:
  CHART_NAME=sample CHART_VERSION=202406.827.0 JFROG_CREDENTIALS=user:password SONARQUBE_TOKEN=595bb1 ./SonarCheck ".*mock,.*-cronjobs,.*-test.*"`)
        os.Exit(0)
}

// extractTarGz extracts a .tgz file into the specified output directory
func ExtractTarGz(gzipPath, outputDir string) error {
        file, err := os.Open(gzipPath)
        if err != nil {
                return err
        }
        defer file.Close()

        gzipReader, err := gzip.NewReader(file)
        if err != nil {
                return err
        }
        defer gzipReader.Close()

        tarReader := tar.NewReader(gzipReader)

        // Create output directory if it doesn't exist
        if err := os.MkdirAll(outputDir, 0755); err != nil {
                return err
        }
        for {
                header, err := tarReader.Next()
                if err == io.EOF {
                        break // End of archive
                }
                if err != nil {
                        return err
                }

                // Create the appropriate file or directory
                outputPath := filepath.Join(outputDir, header.Name)
                switch header.Typeflag {
                case tar.TypeDir:
                        if err := os.MkdirAll(outputPath, 0755); err != nil {
                                return err
                        }
                case tar.TypeReg:
                        // Ensure the directory for the file exists
                        if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
                                return err
                        }
                        outFile, err := os.Create(outputPath)
                        if err != nil {
                                return err
                        }
                        if _, err := io.Copy(outFile, tarReader); err != nil {
                                outFile.Close()
                                return err
                        }
                        outFile.Close()
                default:
                        log.Printf("Ignoring unknown type: %c in %s\n", header.Typeflag, header.Name)
                }
        }
        return nil
}

// cleanWorkingDirectory removes any existing files or directories with the name pattern "layer_*"
func CleanWorkingDirectory(cleaningPattern string, verbose, debug bool) {
	files, err := filepath.Glob(cleaningPattern)
        if err != nil {
                fmt.Println("Error cleaning working directory:", err)
                return
        }

        for _, file := range files {
                err = os.RemoveAll(file)
                if err != nil {
                        fmt.Println("Error removing", file, ":", err)
                } else if verbose || debug {
                        fmt.Println("Removed", file)
                }
        }
}

// extractAppVersions scans the "charts" subdirectories in each layer and extracts the appVersion from Chart.yaml
func ExtractAppVersions(cleaningPattern string) (map[string]string, error) {
	dependencies := make(map[string]string)

        // Look for all "layer_*" directories
        layers, err := filepath.Glob(cleaningPattern)
        if err != nil {
                return nil, err
        }

        for _, layerDir := range layers {
                // Look for Chart.yaml in the "charts" subdirectory
                chartPaths, err := filepath.Glob(filepath.Join(layerDir, "*", "charts", "*", "Chart.yaml"))
                if err != nil {
                        return nil, err
                }

                for _, chartPath := range chartPaths {
                        // Check if the chartPath is a file and not a .tgz file
                        if strings.HasSuffix(chartPath, ".tgz") {
                                continue
                        }
                        chartName := filepath.Base(filepath.Dir(chartPath))
                        appVersion, err := GetAppVersionFromChart(chartPath)
                        if err != nil {
                                log.Printf("Error reading appVersion from %s: %v\n", chartPath, err)
                                continue
                        }
			dependencies[chartName] = appVersion
                }
        }
        return dependencies, nil
}

type Chart struct {
        AppVersion string `yaml:"appVersion"`
}

// getAppVersionFromChart reads the Chart.yaml file and returns the appVersion
func GetAppVersionFromChart(chartPath string) (string, error) {
        data, err := ioutil.ReadFile(chartPath)
        if err != nil {
                return "", err
        }

        var chart Chart
        err = yaml.Unmarshal(data, &chart)
        if err != nil {
                return "", err
        }

        return chart.AppVersion, nil
}

// In order to match the ignore rule, we need to compile the pattern
func CompareWithRegex(ignoreRules, target string) (bool, error) {
        // Split the patterns by comma
        ignoreRuleList := strings.Split(ignoreRules, ",")

        for _, ignoreRule := range ignoreRuleList {
                // Trim whitespace from each pattern
                ignoreRule = strings.TrimSpace(ignoreRule)

                // Add anchors to the pattern to match the entire string
                anchoredIgnoreRule := "^" + ignoreRule + "$"

                // Compile the pattern as a regular expression
                re, err := regexp.Compile(anchoredIgnoreRule)
                if err != nil {
                        return false, fmt.Errorf("[ERROR] compiling regex pattern '%s': %v", ignoreRule, err)
                }

                // Check if the regex matches the target string
                if re.MatchString(target) {
                        return true, nil
                }
        }
        return false, nil
}

// Returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func CompareVersions(v1, v2 string) int {
        v1Int, _ := strconv.Atoi(strings.ReplaceAll(v1, ".", ""))
        v2Int, _ := strconv.Atoi(strings.ReplaceAll(v2, ".", ""))

        if v1Int < v2Int {
                return -1
        } else if v1Int > v2Int {
                return 1
        }
        return 0
}

// Print the status of each component and the whole project status
func LogStatus(dependency, version, status, projectStatus string, verbose, debug bool) string {
    if (verbose || debug) && status == "Passed" {
        log.Printf("%s-%s: %s", dependency, version, status)
    } else if status == "Failed" {
        log.Printf("%s-%s: %s", dependency, version, status)
        projectStatus = "notOK"
    } else if status != "Failed" && status != "Passed" {
        log.Printf("%s-%s: %s (UNKNOWN STATUS)", dependency, version, status)
        projectStatus = "notOK"
    }
    return projectStatus
}

