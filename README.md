# SonarCheck-Status

Check Sonarqube quality gate status to be sure that applications passed the check.

This only works with Jfrog and Sonarqube in a special way.

In case you are using umbrella charts to deliver different component together and you are using Jfrog to hold artifacts and oci chart registry,
you are able to use this code to check the Sonarqube qualitygate status.

There is also Jfrog functions to find dependencies from buildInfo and then extract the chart names in order to fetch the Sonarqube qualitygate status
related to each component.
