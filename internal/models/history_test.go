package models

import "testing"

func TestTaskEventNameConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant TaskEventName
		want     string
	}{
		// Constants defined by coding-3
		{"Planning", TaskEventPlanning, "planning"},
		{"PreExecutionCheckpoint", TaskEventPreExecutionCheckpoint, "pre_execution_checkpoint"},
		{"SubmittedForReview", TaskEventSubmittedForReview, "submitted_for_review"},
		{"Approved", TaskEventApproved, "approved"},
		{"Rejected", TaskEventRejected, "rejected"},
		{"Blocked", TaskEventBlocked, "blocked"},
		{"Merged", TaskEventMerged, "merged"},
		{"Superseded", TaskEventSuperseded, "superseded"},
		{"IntegrationFailed", TaskEventIntegrationFailed, "integration_failed"},
		{"HandoffInitiated", TaskEventHandoffInitiated, "handoff_initiated"},
		{"HandoffResumed", TaskEventHandoffResumed, "handoff_resumed"},
		{"TransitionExecuted", TaskEventTransitionExecuted, "transition_executed"},
		{"TransitionCrashRecov", TaskEventTransitionCrashRecov, "transition_crash_recovery"},
		{"ReviewVerdictApproved", TaskEventReviewVerdictApproved, "review_verdict_approved"},
		{"ReviewVerdictRejected", TaskEventReviewVerdictRejected, "review_verdict_rejected"},
		// Constants added by cp4-remaining-constants
		{"Initialization", TaskEventInitialization, "initialization"},
		{"Created", TaskEventCreated, "created"},
		{"Claimed", TaskEventClaimed, "claimed"},
		{"Abandoned", TaskEventAbandoned, "abandoned"},
		{"ClaimedForIntegrationFix", TaskEventClaimedForIntegrationFix, "claimed_for_integration_fix"},
		{"ClaimReleased", TaskEventClaimReleased, "claim_released"},
		{"ReclaimedAfterRejection", TaskEventReclaimedAfterRejection, "reclaimed_after_rejection"},
		{"ReassignedAfterRejection", TaskEventReassignedAfterRejection, "reassigned_after_rejection"},
		{"WorktreeRecovered", TaskEventWorktreeRecovered, "worktree_recovered"},
		{"DoerClaimReleased", TaskEventDoerClaimReleased, "doer_claim_released"},
		{"ReviewClaimReleased", TaskEventReviewClaimReleased, "review_claim_released"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.want {
				t.Errorf("TaskEvent%s = %q, want %q", tt.name, tt.constant, tt.want)
			}
		})
	}
}
