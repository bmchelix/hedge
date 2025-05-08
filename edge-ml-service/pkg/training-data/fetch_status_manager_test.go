/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package training_data

import (
	"io/ioutil"
	"os"
	"sync"
	"testing"
)

type fields struct {
	status string
	mutex  sync.Mutex
	file   *os.File
}

var flds fields

func TestFetchStatusManager_UpdateStatus(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "status.json")
	if err != nil {
		t.Fatalf("Error creating temporary file: %v", err)
	}

	type args struct {
		status       string
		errorMessage string
	}
	flds = fields{
		status: "IN_PROGRESS",
		mutex:  sync.Mutex{},
		file:   tmpFile,
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		expectedErr bool
	}{
		{"UpdateStatus - Passed", flds, args{"COMPLETED", ""}, false},
		{"UpdateStatus - Passed1", flds, args{"FAILED", "some error"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &FetchStatusManager{
				status: tt.fields.status,
				mutex:  tt.fields.mutex,
				file:   tt.fields.file,
			}
			m.UpdateStatus(tt.args.status, tt.args.errorMessage)
		})
	}
	os.Remove(tmpFile.Name())
}

func TestFetchStatusManager_GetStatus(t *testing.T) {
	flds = fields{
		status: "IN_PROGRESS",
		mutex:  sync.Mutex{},
		file:   nil,
	}
	tests := []struct {
		name          string
		fields        fields
		expectedValue string
	}{
		{"GetStatus - Get initial status", flds, "IN_PROGRESS"},
		{"GetStatus - Get updated status", flds, "COMPLETED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &FetchStatusManager{
				status: tt.fields.status,
				mutex:  tt.fields.mutex,
				file:   tt.fields.file,
			}
			if tt.name == "GetStatus - Get updated status" {
				m.UpdateStatus("COMPLETED")
			}
			status := m.GetStatus()
			if status != tt.expectedValue {
				t.Errorf("Expected status: %s, but got: %s", tt.expectedValue, status)
			}
		})
	}
}

func TestFetchStatusManager_Close(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "status.txt")
	if err != nil {
		t.Fatalf("Error creating temporary file: %v", err)
	}
	m := &FetchStatusManager{
		status: "initialStatus",
		file:   tmpFile,
	}
	m.Close()

	// Check if the file is closed
	if _, err := tmpFile.WriteString("test"); err == nil {
		t.Error("Expected an error when writing to a closed file, but got none.")
	}
	os.Remove(tmpFile.Name())
}

func TestNewFetchStatusManager(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "status.json")
	if err != nil {
		t.Fatalf("Error creating temporary file: %v", err)
	}
	manager := NewFetchStatusManager(tmpFile.Name())

	// Check if the returned manager is not nil
	if manager == nil {
		t.Fatal("NewFetchStatusManager returned nil, expected a valid manager.")
	}

	// Check if the initial status is set correctly
	if manager.GetStatus() != "IN_PROCESS" {
		t.Errorf("Expected initial status 'IN_PROCESS', but got: %s", manager.GetStatus())
	}

	// Check if the file is not nil
	if manager.GetFile() == nil {
		t.Error("File in FetchStatusManager is nil, expected a valid file.")
	}
	os.Remove(tmpFile.Name())
}

func TestFetchStatusManager_GetFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "status.json")
	if err != nil {
		t.Fatalf("Error creating temporary file: %v", err)
	}

	manager := &FetchStatusManager{
		status: "IN_PROGRESS",
		file:   tmpFile,
	}
	file := manager.GetFile()

	// Check if the returned file is not nil
	if file == nil {
		t.Error("GetFile returned nil, expected a valid file.")
	}

	// Check if the returned file matches the original file
	if file.Name() != tmpFile.Name() {
		t.Errorf("Expected file name: %s, but got: %s", tmpFile.Name(), file.Name())
	}
	os.Remove(tmpFile.Name())
}
