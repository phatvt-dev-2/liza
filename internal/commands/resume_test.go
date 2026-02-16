package commands

import (
	"testing"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestResumeCommand(t *testing.T) {
	tests := []struct {
		name        string
		initialMode models.SystemMode
		changedBy   string
		wantErr     bool
		errContains string
		wantMode    models.SystemMode
	}{
		{
			name:        "resume from PAUSED mode",
			initialMode: models.SystemModePaused,
			changedBy:   "human",
			wantErr:     false,
			wantMode:    models.SystemModeRunning,
		},
		{
			name:        "resume from PAUSED with agent ID",
			initialMode: models.SystemModePaused,
			changedBy:   "admin-1",
			wantErr:     false,
			wantMode:    models.SystemModeRunning,
		},
		{
			name:        "cannot resume from RUNNING",
			initialMode: models.SystemModeRunning,
			changedBy:   "human",
			wantErr:     true,
			errContains: "not PAUSED",
			wantMode:    models.SystemModeRunning,
		},
		{
			name:        "cannot resume from STOPPED",
			initialMode: models.SystemModeStopped,
			changedBy:   "human",
			wantErr:     true,
			errContains: "cannot resume from STOPPED",
			wantMode:    models.SystemModeStopped,
		},
		{
			name:        "cannot resume from empty mode",
			initialMode: "",
			changedBy:   "human",
			wantErr:     true,
			errContains: "not PAUSED",
			wantMode:    "",
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

			// Run resume command
			err := ResumeCommand(tmpDir, tt.changedBy)

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
