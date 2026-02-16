package commands

import (
	"testing"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestStartCommand(t *testing.T) {
	tests := []struct {
		name        string
		initialMode models.SystemMode
		reason      string
		changedBy   string
		wantErr     bool
		errContains string
		wantMode    models.SystemMode
	}{
		{
			name:        "start from STOPPED mode",
			initialMode: models.SystemModeStopped,
			reason:      "Beginning work session",
			changedBy:   "human",
			wantErr:     false,
			wantMode:    models.SystemModeRunning,
		},
		{
			name:        "start from STOPPED without reason",
			initialMode: models.SystemModeStopped,
			reason:      "",
			changedBy:   "admin",
			wantErr:     false,
			wantMode:    models.SystemModeRunning,
		},
		{
			name:        "cannot start when already RUNNING",
			initialMode: models.SystemModeRunning,
			reason:      "Test",
			changedBy:   "human",
			wantErr:     true,
			errContains: "already RUNNING",
			wantMode:    models.SystemModeRunning,
		},
		{
			name:        "cannot start when PAUSED - use resume",
			initialMode: models.SystemModePaused,
			reason:      "Test",
			changedBy:   "human",
			wantErr:     true,
			errContains: "PAUSED",
			wantMode:    models.SystemModePaused,
		},
		{
			name:        "cannot start from empty mode (default RUNNING)",
			initialMode: "",
			reason:      "Test",
			changedBy:   "human",
			wantErr:     true,
			errContains: "already RUNNING",
			wantMode:    "", // Mode unchanged on error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			tmpDir := t.TempDir()
			stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

			// Create initial state with specified mode
			state := testhelpers.CreateValidState()
			state.Config.Mode = tt.initialMode

			bb := testhelpers.WriteInitialState(t, stateFile, state)

			// Run start command
			err := StartCommand(tmpDir, tt.reason, tt.changedBy)

			// Check error
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				testhelpers.AssertErrorContains(t, err, tt.errContains)

				// Verify mode didn't change
				updatedState, readErr := bb.Read()
				if readErr != nil {
					t.Fatalf("Failed to read state: %v", readErr)
				}
				if updatedState.Config.Mode != tt.wantMode {
					t.Errorf("Mode should not have changed: got %v, want %v", updatedState.Config.Mode, tt.wantMode)
				}
				return
			}

			testhelpers.AssertNoError(t, err)

			// Verify mode was updated
			updatedState, err := bb.Read()
			if err != nil {
				t.Fatalf("Failed to read state: %v", err)
			}

			if updatedState.Config.Mode != tt.wantMode {
				t.Errorf("Mode = %v, want %v", updatedState.Config.Mode, tt.wantMode)
			}

			// Verify mode_changed_at was set
			if updatedState.Config.ModeChangedAt == nil {
				t.Error("ModeChangedAt should be set")
			}

			// Verify mode_changed_by was set
			if updatedState.Config.ModeChangedBy == nil {
				t.Error("ModeChangedBy should be set")
			} else if *updatedState.Config.ModeChangedBy != tt.changedBy {
				t.Errorf("ModeChangedBy = %v, want %v", *updatedState.Config.ModeChangedBy, tt.changedBy)
			}
		})
	}
}
