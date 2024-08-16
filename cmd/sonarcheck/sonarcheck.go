package main

import (
    "flag"
    "log"

    "sonarcheck/pkg/config"
    "sonarcheck/pkg/sonarqube"
    "sonarcheck/pkg/artifactory"
    "sonarcheck/pkg/utils"
)

func main() {
    // Parse command-line flags
    flag.BoolVar(&config.Verbose, "v", false, "enable verbose output")
    flag.BoolVar(&config.Debug, "d", false, "enable debug output")
    helpFlag := flag.Bool("h", false, "Show help")
    flag.Parse()

    if *helpFlag {
        utils.Help()
    }

    // Load configurations
    config.LoadEnv()

    // Parse arguments
    ignoreRules := flag.Arg(0)

    // Check SonarQube and JFrog server availability
    if err := sonarqube.CheckAvailability(config.SonarQubeURL, config.SonarQubeToken); err != nil {
        log.Fatalf("[ERROR] %v", err)
    }
    if err := artifactory.CheckAvailability(config.JfrogURL); err != nil {
        log.Fatalf("[ERROR] %v", err)
    }
    
    // Fetch oci chart and extract the charts to find out subcharts and versions
    artifactory.FetchOciChart(
        config.JfrogURL, config.OCIRegistry, config.ChartRepository, 
	config.ChartName, config.ChartVersion, config.JfrogCredentials, config.Verbose, config.Debug)
    
    // Extract appVersion from Chart.yaml in each layer's charts subdirectory
    dependencies, err := utils.ExtractAppVersions(config.CleaningPattern)
    if err != nil {
            log.Printf("Error extracting appVersions: %v\n", err)
    }

    if config.Verbose || config.Debug {
            log.Printf("[INFO] These dependencies found from buildInfo: %v\n", dependencies)
    }

    // Check SonarQube status for each component
    projectStatus := "ok"
    for dependency, version := range dependencies {
        // Check if there is an ignore rule for a dependency
        match, err := utils.CompareWithRegex(ignoreRules, dependency)
        if err != nil {
            log.Printf("Error: %v\n", err)
            continue
        } else if match {
            if config.Debug {
                log.Printf("%s: Ignored", dependency)
                log.Printf("Ignore rules: %s, dependency: %s", ignoreRules, dependency)
            } else if config.Verbose {
                log.Printf("%s: Ignored", dependency)
            }
            continue
        }
  
        // Find the project key for the dependency
        projectKey, err := sonarqube.FindProjectKey(dependency, config.SonarQubeURL, config.SonarQubeToken)
        if err != nil {
            log.Printf("%s: Not Found", dependency)
            projectStatus = "notOK"
            continue
        }

        // Check the SonarQube scan status of the dependency
        status, err := sonarqube.SonarCheck(projectKey, version, config.SonarQubeURL, config.SonarQubeToken, config.Debug)
        if err != nil {
            log.Printf("[Error] sending GET request for dependency %s: %v", dependency, err)
            projectStatus = "notOK"
            continue
        }

        // Print the status of SonarQube check
        projectStatus = utils.LogStatus(dependency, version, status, projectStatus, config.Verbose, config.Debug)
    }

    
    // Clean up the working directory before proceeding
    utils.CleanWorkingDirectory(config.CleaningPattern, config.Verbose, config.Debug) 

    if projectStatus == "notOK" {
        log.Fatalf("[ERROR] One or more components failed in SonarQube status check.")
    } else {
        log.Printf("[INFO]: All components passed.")
    }
}

