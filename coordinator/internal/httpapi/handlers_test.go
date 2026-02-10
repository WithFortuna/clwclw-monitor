package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"clwclw-monitor/coordinator/internal/config" // Import config
	"clwclw-monitor/coordinator/internal/model"
	"clwclw-monitor/coordinator/internal/store/memory" // Corrected import
)

// Helper to create a test server with an in-memory store
func newTestServer(t *testing.T) *Server {
	memStore := memory.NewStore() // Corrected NewStore call
	testConfig := config.Config{  // Minimal config for testing
		AuthToken: "test-token",
	}
	return NewServer(testConfig, memStore) // Corrected NewServer call
}

func TestHandleTasks_AutoCreateChain(t *testing.T) {
	ctx := context.Background()
	server := newTestServer(t)

	// 1. Create a channel first, as tasks require a channel_id
	createChannelReq := map[string]string{
		"name": "test-channel",
	}
	channelBody, _ := json.Marshal(createChannelReq)
	channelRec := httptest.NewRecorder()
	channelReq := httptest.NewRequest(http.MethodPost, "/v1/channels", bytes.NewReader(channelBody))
	server.handleChannels(channelRec, channelReq)
	if channelRec.Code != http.StatusCreated {
		t.Fatalf("Failed to create channel: %v", channelRec.Body.String())
	}
	var channelResp map[string]model.Channel
	json.NewDecoder(channelRec.Body).Decode(&channelResp)
	channelID := channelResp["channel"].ID

	// 2. Create a task without specifying a ChainID
	createTaskReq := map[string]any{
		"channel_id":  channelID,
		"title":       "My First Standalone Task",
		"description": "This task should create its own chain.",
		"priority":    0,
	}
	taskBody, _ := json.Marshal(createTaskReq)
	taskRec := httptest.NewRecorder()
	taskReq := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(taskBody))
	server.handleTasks(taskRec, taskReq)

	if taskRec.Code != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusCreated, taskRec.Code, taskRec.Body.String())
	}

	var taskResp map[string]model.Task
	if err := json.NewDecoder(taskRec.Body).Decode(&taskResp); err != nil {
		t.Fatalf("Failed to decode task response: %v", err)
	}
	createdTask := taskResp["task"]

	// Assertions
	if createdTask.ID == "" {
		t.Errorf("Expected created task to have an ID, got empty")
	}
	if createdTask.ChainID == "" {
		t.Errorf("Expected created task to have a ChainID, got empty. Chain was not auto-created.")
	}
	if createdTask.Sequence != 1 {
		t.Errorf("Expected created task to have sequence 1, got %d", createdTask.Sequence)
	}
	if createdTask.Title != "My First Standalone Task" {
		t.Errorf("Expected task title 'My First Standalone Task', got '%s'", createdTask.Title)
	}

	// Verify the auto-created chain actually exists in the store
	fetchedChain, err := server.store.GetChain(ctx, createdTask.ChainID)
	if err != nil {
		t.Fatalf("Failed to fetch auto-created chain %s: %v", createdTask.ChainID, err)
	}

	expectedChainName := fmt.Sprintf("Standalone Chain for %s", strings.TrimSpace("My First Standalone Task"))
	if fetchedChain.Name != expectedChainName {
		t.Errorf("Expected chain name '%s', got '%s'", expectedChainName, fetchedChain.Name)
	}
	if fetchedChain.Status != model.ChainStatusQueued {
		t.Errorf("Expected chain status '%s', got '%s'", model.ChainStatusQueued, fetchedChain.Status)
	}
	if fetchedChain.ChannelID != channelID {
		t.Errorf("Expected chain channel ID '%s', got '%s'", channelID, fetchedChain.ChannelID)
	}
}

func TestHandleTasks_WithProvidedChainID(t *testing.T) {
	server := newTestServer(t)

	// 1. Create a channel
	createChannelReq := map[string]string{
		"name": "test-channel-2",
	}
	channelBody, _ := json.Marshal(createChannelReq)
	channelRec := httptest.NewRecorder()
	channelReq := httptest.NewRequest(http.MethodPost, "/v1/channels", bytes.NewReader(channelBody))
	server.handleChannels(channelRec, channelReq)
	if channelRec.Code != http.StatusCreated {
		t.Fatalf("Failed to create channel: %v", channelRec.Body.String())
	}
	var channelResp map[string]model.Channel
	json.NewDecoder(channelRec.Body).Decode(&channelResp)
	channelID := channelResp["channel"].ID

	// 2. Create a chain
	createChainReq := map[string]any{
		"channel_id":  channelID,
		"name":        "Predefined Chain",
		"description": "A chain created explicitly",
		"status":      "queued",
	}
	chainBody, _ := json.Marshal(createChainReq)
	chainRec := httptest.NewRecorder()
	chainReq := httptest.NewRequest(http.MethodPost, "/v1/chains", bytes.NewReader(chainBody))
	server.handleChains(chainRec, chainReq)
	if chainRec.Code != http.StatusCreated {
		t.Fatalf("Failed to create chain: %v", chainRec.Body.String())
	}
	var chainResp map[string]model.Chain
	json.NewDecoder(chainRec.Body).Decode(&chainResp)
	chainID := chainResp["chain"].ID

	// 3. Create a task with the provided ChainID
	createTaskReq := map[string]any{
		"channel_id":  channelID,
		"chain_id":    chainID,
		"sequence":    5,
		"title":       "Task in Provided Chain",
		"description": "This task should be in the explicitly created chain.",
		"priority":    1,
	}
	taskBody, _ := json.Marshal(createTaskReq)
	taskRec := httptest.NewRecorder()
	taskReq := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(taskBody))
	server.handleTasks(taskRec, taskReq)

	if taskRec.Code != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusCreated, taskRec.Code, taskRec.Body.String())
	}

	var taskResp map[string]model.Task
	if err := json.NewDecoder(taskRec.Body).Decode(&taskResp); err != nil {
		t.Fatalf("Failed to decode task response: %v", err)
	}
	createdTask := taskResp["task"]

	// Assertions
	if createdTask.ChainID != chainID {
		t.Errorf("Expected task to be in chain %s, got %s", chainID, createdTask.ChainID)
	}
	if createdTask.Sequence != 5 {
		t.Errorf("Expected task sequence 5, got %d", createdTask.Sequence)
	}
}

// Add a test for creating a task with a non-existent ChainID
func TestHandleTasks_NonExistentChainID(t *testing.T) {
	server := newTestServer(t)

	// Create a channel first
	createChannelReq := map[string]string{
		"name": "test-channel-3",
	}
	channelBody, _ := json.Marshal(createChannelReq)
	channelRec := httptest.NewRecorder()
	channelReq := httptest.NewRequest(http.MethodPost, "/v1/channels", bytes.NewReader(channelBody))
	server.handleChannels(channelRec, channelReq)
	if channelRec.Code != http.StatusCreated {
		t.Fatalf("Failed to create channel: %v", channelRec.Body.String())
	}
	var channelResp map[string]model.Channel
	json.NewDecoder(channelRec.Body).Decode(&channelResp)
	channelID := channelResp["channel"].ID

	// Attempt to create a task with a non-existent ChainID
	nonExistentChainID := "00000000-0000-0000-0000-000000000000" // A valid UUID format but not in store
	createTaskReq := map[string]any{
		"channel_id":  channelID,
		"chain_id":    nonExistentChainID,
		"title":       "Task for Non-existent Chain",
		"description": "This should fail because the chain does not exist.",
		"priority":    0,
	}
	taskBody, _ := json.Marshal(createTaskReq)
	taskRec := httptest.NewRecorder()
	taskReq := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(taskBody))
	server.handleTasks(taskRec, taskReq)

	if taskRec.Code != http.StatusBadRequest {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusBadRequest, taskRec.Code, taskRec.Body.String())
	}

	if !strings.Contains(taskRec.Body.String(), "chain_id_not_found") {
		t.Errorf("Expected error message containing 'chain_id_not_found', got '%s'", taskRec.Body.String())
	}
}
