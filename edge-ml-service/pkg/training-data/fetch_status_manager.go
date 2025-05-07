/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package training_data

import (
	"encoding/json"
	"fmt"
	"hedge/edge-ml-service/pkg/helpers"
	"os"
	"path/filepath"
	"sync"
)

const (
	StatusInProgress = "IN_PROGRESS"
	StatusFailed     = "FAILED"
	StatusCompleted  = "COMPLETED"
	StatusDeprecated = "DEPRECATED"
)

type FetchStatusManagerInterface interface {
	UpdateStatus(status string, errorMessage ...string)
	GetStatus() string
	GetFile() *os.File
	Close()
}

type StatusResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

type FetchStatusManager struct {
	status string
	mutex  sync.Mutex
	file   *os.File
}

func NewFetchStatusManager(statusFileName string) FetchStatusManagerInterface {
	// Create the necessary directories (if they don't exist)
	dir := filepath.Dir(statusFileName)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		fmt.Println("Error creating directories:", err)
		return nil
	}

	file, err := os.Create(statusFileName)
	if err != nil {
		return nil
	}

	return &FetchStatusManager{
		status: "IN_PROCESS",
		file:   file,
	}
}

func ReadStatus(filePath string) (*StatusResponse, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var status StatusResponse
	err = json.Unmarshal(data, &status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func UpdateStatus(filePath string, status *StatusResponse) error {
	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	err = helpers.SaveFile(filePath, data)
	if err != nil {
		return err
	}
	return nil
}

// UpdateStatus safely updates the fetch status and writes the JSON to file
func (m *FetchStatusManager) UpdateStatus(status string, errorMessage ...string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.status = status
	response := StatusResponse{
		Status: status,
	}
	if status == StatusFailed && len(errorMessage) > 0 {
		response.ErrorMessage = errorMessage[0]
	}
	statusJSON, err := json.Marshal(response)
	if err != nil {
		fmt.Println("Error marshaling status:", err)
		return
	}
	m.file.Seek(0, 0)
	m.file.Truncate(0)
	m.file.Write(statusJSON)
}

// GetStatus safely returns the current fetch status
func (m *FetchStatusManager) GetStatus() string {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.status
}

func (m *FetchStatusManager) GetFile() *os.File {
	return m.file
}

// Close the status file when done
func (m *FetchStatusManager) Close() {
	m.file.Close()
}
