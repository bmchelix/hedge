#!/bin/sh

#set -x

if [[ "$sonar_token_go" == "" ]]; then
  echo -e "\nERROR: \"sonar_token_go\" environment not set. This auth token for \"TEST-SONAR-HEDGE-BE\" sonarqube project is required by sonar-scanner. Skipping BE coverage.."
else
  echo -e "\n\n=============================================="
  echo -e     "========== Run sonar-scanner for BE =========="
  echo -e     "=============================================="
  cp sonarqube/sonar-project.properties_go ./sonar-project.properties
  sonar-scanner -Dsonar.token=$sonar_token_go
fi

if [[ "$sonar_token_py" == "" ]]; then
  echo -e "\nERROR: \"sonar_token_py\" environment not set. This auth token for \"TEST-SONAR-HEDGE-PY\" sonarqube project is required by sonar-scanner. Skipping PY coverage.."
else
  echo -e "\n\n=============================================="
  echo -e     "========== Run sonar-scanner for PY =========="
  echo -e     "=============================================="
  cp sonarqube/sonar-project.properties_py ./sonar-project.properties
  sonar-scanner -Dsonar.token=$sonar_token_py
fi

if [[ "$sonar_token_ui" == "" ]]; then
  echo -e "\nERROR: \"sonar_token_ui\" environment not set. This auth token for \"TEST-SONAR-HEDGE-UI\" sonarqube project is required by sonar-scanner. Skipping UI coverage.."
else
  echo -e "\n\n=============================================="
  echo -e     "========== Run sonar-scanner for UI =========="
  echo -e     "=============================================="
  cp sonarqube/sonar-project.properties_ui ./sonar-project.properties
  sonar-scanner -Dsonar.token=$sonar_token_ui
fi

if [[ "$sonar_token_all" == "" ]]; then
  echo -e "\nERROR: \"sonar_token_all\" environment not set. This auth token for \"TEST-SONAR-HEDGE-ALL\" sonarqube project is required by sonar-scanner. Skipping OVERALL coverage.."
else
  echo -e "\n\n==============================================="
  echo -e     "========== Run sonar-scanner for ALL =========="
  echo -e     "==============================================="
  cp sonarqube/sonar-project.properties ./sonar-project.properties
  sonar-scanner -Dsonar.token=$sonar_token_all
fi