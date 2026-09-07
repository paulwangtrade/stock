package diagnostic

import (
	"go-stock/backend/version"
	"strings"
)

// BuildBlock summarizes build identity for bundle export.
type BuildBlock struct {
	VersionInfo     version.VersionInfo `json:"version_info"`
	GOOS            string              `json:"goos"`
	GOARCH          string              `json:"goarch"`
	BuildMode       string              `json:"build_mode"`
	IdentityUnknown bool                `json:"identity_unknown"`
}

func collectBuild(v version.VersionInfo, env Environment) BuildBlock {
	unknown := strings.TrimSpace(v.GitCommit) == "" ||
		strings.EqualFold(strings.TrimSpace(v.GitCommit), "unknown") ||
		strings.TrimSpace(v.BuildTime) == "" ||
		strings.EqualFold(strings.TrimSpace(v.BuildTime), "unknown")
	return BuildBlock{
		VersionInfo:     v,
		GOOS:            env.OS,
		GOARCH:          env.Arch,
		BuildMode:       v.BuildMode,
		IdentityUnknown: unknown,
	}
}

// VersionBlock is the bundle-level version projection.
type VersionBlock struct {
	Version    string              `json:"version"`
	BuildTime  string              `json:"build_time"`
	GitCommit  string              `json:"git_commit"`
	CommitHash string              `json:"commit_hash"`
	Channel    string              `json:"channel"`
	BuildMode  string              `json:"build_mode"`
	Info       version.VersionInfo `json:"info"`
}

func collectVersionBlock(v version.VersionInfo) VersionBlock {
	return VersionBlock{
		Version:    v.Version,
		BuildTime:  v.BuildTime,
		GitCommit:  v.GitCommit,
		CommitHash: v.GitCommit,
		Channel:    v.Channel,
		BuildMode:  v.BuildMode,
		Info:       v,
	}
}
