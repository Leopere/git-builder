package gitops

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type deployState struct {
	LastSuccessSHA string `json:"last_success_sha"`
}

func deployStatePath(workdir, repoURL string) string {
	return filepath.Join(workdir, ".git-builder-state", RepoDirName(repoURL)+".json")
}

// IsDeployed reports whether this full commit hash was already successfully deployed for the repo.
func IsDeployed(workdir, repoURL, headFullHex string) bool {
	b, err := os.ReadFile(deployStatePath(workdir, repoURL))
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
func SetDeployed(workdir, repoURL, headFullHex string) error {
	p := deployStatePath(workdir, repoURL)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	b, err := json.Marshal(deployState{LastSuccessSHA: headFullHex})
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0644)
}
