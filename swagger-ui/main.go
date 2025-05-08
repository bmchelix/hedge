package main

import (
	"encoding/json"
	"fmt"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/labstack/echo/v4"
	"github.com/pelletier/go-toml"
	"os"
	"path/filepath"
	"strconv"
)

type swaggerConfig struct {
	port int64 `toml:"Port"`
}

var lc = logger.NewClient("hedge-swagger-ui", "INFO")

func main() {
	workingDir, err := os.Getwd()
	if err != nil {
		lc.Errorf("Failed to get current working directory: %v", err)
	}

	lc.Infof("Working directory: %s", workingDir)

	config, err := readToml(workingDir)

	if err != nil {
		lc.Errorf("Error reading config: %v", err)
		return
	}

	lc.Infof("Swagger UI port: %d", config.port)

	baseUrl := os.Getenv("BASE_URL")
	if baseUrl != "" {
		changeHostnameInSwaggerFile(filepath.Join(workingDir, "res", "swagger", "swagger.json"), baseUrl)
	}

	e := echo.New()
	e.Static("/", filepath.Join(workingDir, "res", "swagger"))

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", config.port)))
}

func readToml(workingDir string) (*swaggerConfig, error) {
	config := &swaggerConfig{}
	configFilePath := filepath.Join(workingDir, "res", "configuration.toml")

	lc.Infof("Loading swagger config from: %s", configFilePath)

	configFile, err := toml.LoadFile(configFilePath)
	if err != nil {
		return nil, err
	}

	port := configFile.Get("Port").(int64)
	config.port = port
	return config, nil
}

func changeHostnameInSwaggerFile(filePath string, baseUrl string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		lc.Errorf("Error reading file: %v", err)
		os.Exit(1)
	}

	var jsonData map[string]interface{}
	err = json.Unmarshal(data, &jsonData)
	if err != nil {
		lc.Errorf("Error parsing JSON: %v", err)
		os.Exit(1)
	}

	isExternalAuth, err := strconv.ParseBool(os.Getenv("IS_EXTERNAL_AUTH"))
	if err != nil {
		isExternalAuth = false
	}
	if isExternalAuth {
		domain := os.Getenv("DOMAIN")
		jsonData["host"] = baseUrl + domain
	} else {
		nginxPort := os.Getenv("NGINX_PORT")
		if nginxPort == "" {
			nginxPort = "80"
		}
		jsonData["host"] = baseUrl + ":" + nginxPort
	}

	if isExternalAuth {
		delete(jsonData, "security")
		delete(jsonData, "securityDefinitions")
	}

	updatedData, err := json.MarshalIndent(jsonData, "", "\t")
	if err != nil {
		lc.Errorf("Error marshaling JSON: %v", err)
		os.Exit(1)
	}

	err = os.WriteFile(filePath, updatedData, 0644)
	if err != nil {
		lc.Errorf("Error writing file: %v", err)
		os.Exit(1)
	}
}
