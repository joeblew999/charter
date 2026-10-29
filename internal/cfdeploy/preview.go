package cfdeploy

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"regexp"
	"slices"

	"github.com/joeblew999/charter/internal/cfapi"
)

// Worker Previews (https://developers.cloudflare.com/workers/previews/): named copies of a Worker, each with its
// own code, bindings, URL and Durable Object namespace and storage, under the same Worker as production. A preview
// is typically one per pull request. These are the calls `wrangler preview` makes (wrangler's
// deploy-helpers/src/preview/api.ts), made with the REST API:
//
//  1. turn previews on for the Worker (its workers.dev subdomain settings)
//  2. D1: as Deploy (find or create each database, apply migrations) — point previews at their own database
//  3. find the preview by name, or create it (POST …/workers/workers/{worker}/previews)
//  4. the preview's Durable Object migration tag (GET …/previews/{id}/deployments/latest), as Deploy reads the
//     script's: a new preview starts with none, so its first deployment creates the classes
//  5. Static Assets upload, as Deploy
//  6. POST …/previews/{id}/deployments: multipart metadata (main module, compatibility date, bindings,
//     migrations, assets JWT, annotations) + one "files" part per module

// previewNameRe is the API's rule for a preview name.
var previewNameRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-_/.+=]*[a-zA-Z0-9])?$`)

// PreviewResult is where a deployed preview answers.
type PreviewResult struct {
	ID   string   // the preview's immutable id
	URLs []string // stable preview URLs (same across deployments), then this deployment's own URLs
}

// URL is the preview's main URL ("" if Cloudflare returned none).
func (r PreviewResult) URL() string {
	if len(r.URLs) == 0 {
		return ""
	}
	return r.URLs[0]
}

type preview struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	URLs []string `json:"urls"`
}

// DeployPreview deploys cfg as the preview named name of the existing Worker cfg.Name. Annotations (keys such as
// "workers/pull_request_number", "workers/commit_sha") label the deployment in the dashboard. Custom domains in
// cfg are ignored: a preview answers on its workers.dev preview URL.
func (c *Client) DeployPreview(ctx context.Context, cfg Config, name string, annotations map[string]string) (PreviewResult, error) {
	if !previewNameRe.MatchString(name) || len(name) > 256 {
		return PreviewResult{}, fmt.Errorf("cfdeploy: preview name %q: letters, digits and - _ / . + =, starting and ending with a letter or digit", name)
	}
	plan, err := NewPlan(cfg)
	if err != nil {
		return PreviewResult{}, err
	}
	if c.Token == "" || c.AccountID == "" {
		return PreviewResult{}, errors.New("cfdeploy: Token and AccountID are required")
	}
	exists, _, err := c.script(ctx, cfg.Name)
	if err != nil {
		return PreviewResult{}, fmt.Errorf("cfdeploy: list scripts: %w", err)
	}
	if !exists {
		return PreviewResult{}, fmt.Errorf("cfdeploy: no Worker named %q: deploy production once before previews", cfg.Name)
	}

	// 1. Previews must be on, or the preview URLs don't answer.
	if err := c.do(ctx, "POST", c.accountPath("/workers/scripts/"+cfg.Name+"/subdomain"), "",
		cfapi.JSON(map[string]bool{"enabled": true, "previews_enabled": true}), nil); err != nil {
		return PreviewResult{}, fmt.Errorf("cfdeploy: enable previews: %w", err)
	}

	// 2. D1.
	d1IDs := map[string]string{}
	for _, binding := range slices.Sorted(maps.Keys(cfg.D1)) {
		ref := cfg.D1[binding]
		id, err := c.resolveD1(ctx, ref, cfg.D1Create)
		if err != nil {
			return PreviewResult{}, fmt.Errorf("cfdeploy: d1 %s=%s: %w", binding, ref, err)
		}
		d1IDs[binding] = id
		if cfg.MigrationsDir != "" {
			if err := c.migrateD1(ctx, id, cfg.MigrationsDir); err != nil {
				return PreviewResult{}, fmt.Errorf("cfdeploy: d1 %s migrations: %w", ref, err)
			}
		}
	}

	// 3. The preview.
	base := c.accountPath("/workers/workers/" + cfg.Name + "/previews")
	var pv preview
	found, err := c.optional(ctx, base+"/"+url.PathEscape(name), &pv)
	if err != nil {
		return PreviewResult{}, fmt.Errorf("cfdeploy: preview %s: %w", name, err)
	}
	// Workers Logs, as production has: a preview's own setting, off unless set.
	logs := map[string]any{"enabled": true, "head_sampling_rate": 1}
	if !found {
		if err := c.do(ctx, "POST", base, "", cfapi.JSON(map[string]any{"name": name, "observability": logs}), &pv); err != nil {
			return PreviewResult{}, fmt.Errorf("cfdeploy: create preview %s: %w", name, err)
		}
		c.logf("✓ preview %s created (%s)", name, pv.ID)
	} else {
		if err := c.do(ctx, "PATCH", base+"/"+pv.ID, "", cfapi.JSON(map[string]any{"observability": logs}), nil); err != nil {
			c.logf("· preview %s: Workers Logs not switched on: %v", name, err)
		}
		c.logf("✓ preview %s (%s)", name, pv.ID)
	}
	if pv.ID == "" {
		return PreviewResult{}, fmt.Errorf("cfdeploy: preview %s: the API returned no id", name)
	}

	// 4. Durable Object migration, per preview.
	var doMigration map[string]any
	if cfg.MigrationTag != "" {
		var latest struct {
			MigrationTag string `json:"migration_tag"`
		}
		if _, err := c.optional(ctx, base+"/"+pv.ID+"/deployments/latest", &latest); err != nil {
			return PreviewResult{}, fmt.Errorf("cfdeploy: preview %s latest deployment: %w", name, err)
		}
		switch latest.MigrationTag {
		case cfg.MigrationTag:
		case "":
			doMigration = map[string]any{"new_tag": cfg.MigrationTag, "new_sqlite_classes": cfg.NewSQLiteClasses}
		default:
			return PreviewResult{}, fmt.Errorf("cfdeploy: preview %s is at migration %q; moving to %q needs an explicit migration step", name, latest.MigrationTag, cfg.MigrationTag)
		}
	}

	// 5. Static Assets.
	jwt := ""
	if len(plan.Assets) > 0 {
		if jwt, err = c.uploadAssets(ctx, cfg.Name, plan); err != nil {
			return PreviewResult{}, fmt.Errorf("cfdeploy: assets: %w", err)
		}
	}

	// 6. The deployment.
	meta := map[string]any{
		"main_module":        plan.Modules[0].Name,
		"compatibility_date": cfg.CompatibilityDate,
		"bindings":           bindings(cfg, d1IDs),
	}
	if doMigration != nil {
		meta["migrations"] = doMigration
	}
	if jwt != "" {
		meta["assets"] = map[string]string{"jwt": jwt}
	}
	if len(annotations) > 0 {
		meta["annotations"] = annotations
	}
	body, err := scriptBody(meta, plan.Modules, func(Module) string { return "files" })
	if err != nil {
		return PreviewResult{}, err
	}
	var dep struct {
		URLs []string `json:"urls"`
	}
	if err := c.do(ctx, "POST", base+"/"+pv.ID+"/deployments", "", body, &dep); err != nil {
		return PreviewResult{}, fmt.Errorf("cfdeploy: preview %s deployment: %w", name, err)
	}
	c.logf("✓ preview %s deployed (%d modules)", name, len(plan.Modules))

	// The stable URLs come with the preview; read it again if it was created without them.
	if len(pv.URLs) == 0 {
		if _, err := c.optional(ctx, base+"/"+pv.ID, &pv); err != nil {
			return PreviewResult{}, fmt.Errorf("cfdeploy: preview %s: %w", name, err)
		}
	}
	return PreviewResult{ID: pv.ID, URLs: append(slices.Clone(pv.URLs), dep.URLs...)}, nil
}

// DeletePreview removes the preview named name of the Worker worker, with its Durable Object storage. It reports
// whether there was one.
func (c *Client) DeletePreview(ctx context.Context, worker, name string) (bool, error) {
	if c.Token == "" || c.AccountID == "" {
		return false, errors.New("cfdeploy: Token and AccountID are required")
	}
	p := c.accountPath("/workers/workers/" + worker + "/previews/" + url.PathEscape(name))
	var pv preview
	found, err := c.optional(ctx, p, &pv)
	if err != nil || !found {
		return false, err
	}
	if err := c.do(ctx, "DELETE", p, "", nil, nil); err != nil {
		return false, fmt.Errorf("cfdeploy: delete preview %s: %w", name, err)
	}
	c.logf("✓ preview %s deleted", name)
	return true, nil
}

// optional GETs apiPath into out, reporting false (and no error) when the API says it doesn't exist.
func (c *Client) optional(ctx context.Context, apiPath string, out any) (bool, error) {
	err := c.do(ctx, "GET", apiPath, "", nil, out)
	if err == nil {
		return true, nil
	}
	if cfapi.IsNotFound(err) {
		return false, nil
	}
	return false, err
}

// PrunePreviews deletes the previews of the Worker worker whose names keep doesn't accept (e.g. branches that no
// longer exist), with their Durable Object storage, and returns the deleted names.
func (c *Client) PrunePreviews(ctx context.Context, worker string, keep func(name string) bool) ([]string, error) {
	if c.Token == "" || c.AccountID == "" {
		return nil, errors.New("cfdeploy: Token and AccountID are required")
	}
	var pvs []preview
	if err := c.do(ctx, "GET", c.accountPath("/workers/workers/"+worker+"/previews"), "", nil, &pvs); err != nil {
		return nil, fmt.Errorf("cfdeploy: list previews: %w", err)
	}
	var deleted []string
	for _, pv := range pvs {
		if keep(pv.Name) {
			continue
		}
		if err := c.do(ctx, "DELETE", c.accountPath("/workers/workers/"+worker+"/previews/"+pv.ID), "", nil, nil); err != nil {
			return deleted, fmt.Errorf("cfdeploy: delete preview %s: %w", pv.Name, err)
		}
		c.logf("✓ preview %s deleted (branch gone)", pv.Name)
		deleted = append(deleted, pv.Name)
	}
	return deleted, nil
}
