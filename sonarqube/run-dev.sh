#!/bin/sh

cp sonarqube/sonar-project.properties_go.dev ./sonar-project.properties
sonar-scanner -Dsonar_token=devtest

sleep infinity & wait
