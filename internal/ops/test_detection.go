package ops

import (
	"strings"

	"github.com/liza-mas/liza/internal/git"
)

// HasTestFiles checks whether the commits between baseCommit and HEAD in the
// task worktree include any *_test.go files (added or modified).
func HasTestFiles(g *git.Git, taskID, baseCommit string) (bool, error) {
	wtPath := g.GetWorktreePath(taskID)
	files, err := g.DiffFiles(wtPath, baseCommit, "HEAD")
	if err != nil {
		return false, err
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			return true, nil
		}
	}
	return false, nil
}
