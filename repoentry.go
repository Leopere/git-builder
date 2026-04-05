package main

import "git-builder/config"

func repoDisplay(repo config.Repo) string {
	if branch := repo.BranchName(); branch != "" {
		return repo.URL + "@" + branch
	}
	return repo.URL
}

func matchingRepos(repos []config.Repo, url string) []config.Repo {
	out := make([]config.Repo, 0, len(repos))
	for _, repo := range repos {
		if repo.URL == url {
			out = append(out, repo)
		}
	}
	return out
}
