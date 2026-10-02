package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Every API has the same groups in the same order, so a task that takes <api> <group> works for
// each of them. The showcases add one: typescript-public, which shows Fern's audiences.
func TestEveryAPIHasTheCommonGroups(t *testing.T) {
	files, err := filepath.Glob("../sdk/fern/apis/*/generators.yml")
	if err != nil {
		t.Fatal(err)
	}
	var apis []string
	for _, file := range files {
		api := filepath.Base(filepath.Dir(file))
		apis = append(apis, api)
		groups, err := sdkGroups(file)
		if err != nil {
			t.Fatal(err)
		}
		want := sdkCommon
		if strings.HasPrefix(api, "showcase-") {
			want = append(slices.Clone(sdkCommon), "typescript-public")
		}
		if !slices.Equal(groups, want) {
			t.Errorf("%s: groups %v, want %v", api, groups, want)
		}
	}
	if want := []string{"api-go", "api-ts", "showcase-go", "showcase-ts"}; !slices.Equal(apis, want) {
		t.Errorf("sdk/fern/apis has %v, want %v", apis, want)
	}
}

// sdk-check and cli-build take an API the way sdk-gen does, and say how to generate what is missing.
func TestSDKCommandsTakeAnAPI(t *testing.T) {
	for want, err := range map[string]error{
		"sdk-check needs <api> <group>": sdkCheck([]string{"sdk/out/api-go/go"}),
		"mise run sdk:gen nope go":      sdkCheck([]string{"nope", "go"}),
		"cli-build needs <api>":         cliBuild(nil),
		"mise run sdk:gen nope cli":     cliBuild([]string{"nope"}),
		"sdk-gen needs <api> <group>":   sdkGen([]string{"api-go"}),
		"sdk-ready needs <api>":         sdkReady(nil),
		"sdk-publish needs <api>":       sdkPublish(nil),
		"dist-sdk needs <api>...":       distSDK(nil),
		"dist-cli needs <api>...":       distCLI(nil),
	} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("got %v, want an error with %q", err, want)
		}
	}
}
