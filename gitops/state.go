package gitops

import (
	"encoding/json"
	"os"
	"path/filepath"

	"git-builder/config"
)

type deployState struct {
	LastSuccessSHA string `json:"last_success_sha"`
}

func deployStatePath(workdir string, repo config.Repo) string {
	return filepath.Join(workdir, ".git-builder-state", RepoWorkdirName(repo)+".json")
}

// IsDeployed reports whether this full commit hash was already successfully deployed for the repo.
func IsDeployed(workdir string, repo config.Repo, headFullHex string) bool {
	b, err := os.ReadFile(deployStatePath(workdir, repo))
	if err != nil {
		return false
	}
	var s deployState
	if err := json.Unmarshal(b, &s); err != nil {
		return false
	}
	return s.LastSuccessSHA == headFullHex
}

// SetDeployed records that headFullHex was successfully deployed (script OK or no script needed).
func SetDeployed(workdir string, repo config.Repo, headFullHex string) error {
	p := deployStatePath(workdir, repo)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	b, err := json.Marshal(deployState{LastSuccessSHA: headFullHex})
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0644)
}
