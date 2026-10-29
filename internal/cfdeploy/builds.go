package cfdeploy

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/joeblew999/charter/internal/cfapi"
)

// Workers Builds (Cloudflare's GitHub app) as code: ConfigureBuilds makes the Worker's build setup match a
// BuildsConfig, creating or updating what's there, so it lives in the repo instead of in someone's dashboard
// clicks. The calls are the Builds REST API's.

// BuildsConfig is the Workers Builds setup for one Worker and one GitHub repository.
type BuildsConfig struct {
	Worker string // Worker script name

	// The GitHub repository and its numeric ids (GET https://api.github.com/repos/{owner}/{repo}: .owner.id, .id).
	Owner, Repo     string
	OwnerID, RepoID string

	// BuildToken is the name of a Workers Builds token in the account. The API can't create one (it needs an API
	// token's secret): make it once in the dashboard. FallbackBuildToken, if set, is used (with a warning) when
	// BuildToken doesn't exist yet.
	BuildToken, FallbackBuildToken string

	// The preview trigger: every branch except ProductionBranch runs BuildCommand then DeployCommand, with Vars as
	// build variables. PathExcludes skips pushes that only change matching paths.
	ProductionBranch            string // default "main"
	BuildCommand, DeployCommand string
	Vars                        map[string]string
	PathExcludes                []string
}

// PreviewTriggerName is the name of the trigger ConfigureBuilds manages.
func (b BuildsConfig) PreviewTriggerName() string { return b.Worker + " branch previews" }

type buildToken struct {
	UUID string `json:"build_token_uuid"`
	Name string `json:"build_token_name"`
}

type buildTrigger struct {
	UUID           string   `json:"trigger_uuid"`
	Name           string   `json:"trigger_name"`
	BranchIncludes []string `json:"branch_includes"`
}

// ConfigureBuilds makes the Worker's Workers Builds setup match cfg and returns the preview trigger's id. The
// production branch's trigger runs the same commands, which decide what that branch deploys (a staging Worker, say).
func (c *Client) ConfigureBuilds(ctx context.Context, cfg BuildsConfig) (string, error) {
	if cfg.Worker == "" || cfg.Owner == "" || cfg.Repo == "" || cfg.OwnerID == "" || cfg.RepoID == "" ||
		cfg.BuildCommand == "" || cfg.DeployCommand == "" {
		return "", errors.New("cfdeploy: builds: Worker, Owner, Repo, OwnerID, RepoID, BuildCommand and DeployCommand are required")
	}
	if c.Token == "" || c.AccountID == "" {
		return "", errors.New("cfdeploy: Token and AccountID are required")
	}
	prod := cfg.ProductionBranch
	if prod == "" {
		prod = "main"
	}

	tag, err := c.scriptTag(ctx, cfg.Worker)
	if err != nil {
		return "", err
	}

	var tokens []buildToken
	if err := c.do(ctx, "GET", c.accountPath("/builds/tokens"), "", nil, &tokens); err != nil {
		return "", fmt.Errorf("cfdeploy: builds: list build tokens: %w", err)
	}
	token := ""
	for _, name := range []string{cfg.BuildToken, cfg.FallbackBuildToken} {
		for _, t := range tokens {
			if name != "" && t.Name == name {
				token = t.UUID
				if name != cfg.BuildToken {
					c.logf("! build token %q doesn't exist yet; using %q. Create %q in the dashboard (Workers & Pages → Settings → Builds → API tokens)", cfg.BuildToken, name, cfg.BuildToken)
				}
				break
			}
		}
		if token != "" {
			break
		}
	}
	if token == "" {
		return "", fmt.Errorf("cfdeploy: builds: no build token named %q: create it once in the dashboard (the API can't)", cfg.BuildToken)
	}

	var conn struct {
		UUID string `json:"repo_connection_uuid"`
	}
	if err := c.do(ctx, "PUT", c.accountPath("/builds/repos/connections"), "", cfapi.JSON(map[string]string{
		"provider_type": "github", "provider_account_id": cfg.OwnerID, "provider_account_name": cfg.Owner,
		"repo_id": cfg.RepoID, "repo_name": cfg.Repo,
	}), &conn); err != nil {
		return "", fmt.Errorf("cfdeploy: builds: repository connection: %w", err)
	}
	c.logf("✓ repository %s/%s connected", cfg.Owner, cfg.Repo)

	var triggers []buildTrigger
	if err := c.do(ctx, "GET", c.accountPath("/builds/workers/"+tag+"/triggers"), "", nil, &triggers); err != nil && !cfapi.IsNotFound(err) {
		return "", fmt.Errorf("cfdeploy: builds: list triggers: %w", err)
	}
	if len(triggers) == 0 {
		// A Worker without a build configuration gets one; its production-branch trigger is set below.
		echo := map[string]any{
			"build_command": cfg.BuildCommand, "deploy_command": cfg.DeployCommand, "build_token_uuid": token, "root_directory": "/",
		}
		if err := c.do(ctx, "POST", c.accountPath("/builds/workers"), "", cfapi.JSON(map[string]any{
			"script_tag": tag, "previews_enabled": false, "production_settings": echo, "previews_base_config": echo,
			"git_repository": map[string]string{"branch": prod, "provider_type": "github", "provider_account_id": cfg.OwnerID,
				"provider_account_name": cfg.Owner, "repo_id": cfg.RepoID, "repo_name": cfg.Repo},
		}), nil); err != nil {
			return "", fmt.Errorf("cfdeploy: builds: create build configuration: %w", err)
		}
		c.logf("✓ build configuration created")
		if err := c.do(ctx, "GET", c.accountPath("/builds/workers/"+tag+"/triggers"), "", nil, &triggers); err != nil {
			return "", fmt.Errorf("cfdeploy: builds: list triggers: %w", err)
		}
	}

	excludes := cfg.PathExcludes
	if excludes == nil {
		excludes = []string{}
	}
	trig := map[string]any{
		"trigger_name": cfg.PreviewTriggerName(), "build_command": cfg.BuildCommand, "deploy_command": cfg.DeployCommand,
		"build_token_uuid": token, "root_directory": "/", "branch_includes": []string{"*"}, "branch_excludes": []string{prod},
		"path_includes": []string{"*"}, "path_excludes": excludes, "build_caching_enabled": true,
		"repo_connection_uuid": conn.UUID,
	}
	id := ""
	for _, t := range triggers {
		if t.Name == cfg.PreviewTriggerName() {
			id = t.UUID
		}
	}
	if id == "" {
		trig["external_script_id"] = tag
		var created buildTrigger
		if err := c.do(ctx, "POST", c.accountPath("/builds/triggers"), "", cfapi.JSON(trig), &created); err != nil {
			return "", fmt.Errorf("cfdeploy: builds: create preview trigger: %w", err)
		}
		id = created.UUID
		c.logf("✓ trigger %q created", cfg.PreviewTriggerName())
	} else {
		if err := c.do(ctx, "PATCH", c.accountPath("/builds/triggers/"+id), "", cfapi.JSON(trig), nil); err != nil {
			return "", fmt.Errorf("cfdeploy: builds: update preview trigger: %w", err)
		}
		c.logf("✓ trigger %q updated", cfg.PreviewTriggerName())
	}
	// The production branch's trigger: the same build and deploy, so it keeps a staging preview of that branch.
	ids := []string{id}
	for _, t := range triggers {
		if t.Name != cfg.PreviewTriggerName() && slices.Contains(t.BranchIncludes, prod) {
			if err := c.do(ctx, "PATCH", c.accountPath("/builds/triggers/"+t.UUID), "", cfapi.JSON(map[string]any{
				"build_command": cfg.BuildCommand, "deploy_command": cfg.DeployCommand, "build_token_uuid": token,
				"path_excludes": excludes, "build_caching_enabled": true,
			}), nil); err != nil {
				return "", fmt.Errorf("cfdeploy: builds: update %s trigger: %w", prod, err)
			}
			c.logf("✓ %s trigger deploys the %s preview (staging)", prod, prod)
			ids = append(ids, t.UUID)
		}
	}
	if len(cfg.Vars) > 0 {
		vars := map[string]any{}
		for _, k := range slices.Sorted(maps.Keys(cfg.Vars)) {
			vars[k] = map[string]any{"value": cfg.Vars[k], "is_secret": false}
		}
		for _, tid := range ids {
			if err := c.do(ctx, "PATCH", c.accountPath("/builds/triggers/"+tid+"/environment_variables"), "", cfapi.JSON(vars), nil); err != nil {
				return "", fmt.Errorf("cfdeploy: builds: build variables: %w", err)
			}
		}
		c.logf("✓ build variables %v", slices.Sorted(maps.Keys(cfg.Vars)))
	}
	return id, nil
}

// scriptTag is the system tag of the Worker name (the Builds API keys Workers by it).
func (c *Client) scriptTag(ctx context.Context, name string) (string, error) {
	var scripts []struct {
		ID  string `json:"id"`
		Tag string `json:"tag"`
	}
	if err := c.do(ctx, "GET", c.accountPath("/workers/scripts"), "", nil, &scripts); err != nil {
		return "", fmt.Errorf("cfdeploy: list scripts: %w", err)
	}
	for _, s := range scripts {
		if s.ID == name {
			if s.Tag == "" {
				return "", fmt.Errorf("cfdeploy: Worker %q has no tag", name)
			}
			return s.Tag, nil
		}
	}
	return "", fmt.Errorf("cfdeploy: no Worker named %q: deploy it once first", name)
}
