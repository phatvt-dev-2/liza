package commands

import (
	"fmt"

	"github.com/liza-mas/liza/internal/ops"
)

// SetDiscoveryDispositionCommand sets the disposition of a discovered item.
// Delegates business logic to ops.SetDiscoveryDisposition.
func SetDiscoveryDispositionCommand(projectRoot, discoveryID, disposition string) error {
	if err := ops.SetDiscoveryDisposition(projectRoot, discoveryID, disposition); err != nil {
		return fmt.Errorf("set discovery disposition: %w", err)
	}

	fmt.Printf("Discovery %s disposition set to %s\n", discoveryID, disposition)
	return nil
}
