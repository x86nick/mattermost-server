// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package utils_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils"
)

func mustReadTestFile(t *testing.T, filename string) string {
	contents, err := ioutil.ReadFile(filepath.Join("..", "tests", filename))
	require.NoError(t, err)
	return string(contents)
}

func TestUpdateAssetsSubpathFromConfig(t *testing.T) {
	t.Run("dev build", func(t *testing.T) {
		var oldBuildNumber = model.BuildNumber
		model.BuildNumber = "dev"
		defer func() {
			model.BuildNumber = oldBuildNumber
		}()

		err := utils.UpdateAssetsSubpathFromConfig(nil)
		require.NoError(t, err)
	})

	t.Run("IS_CI=true", func(t *testing.T) {
		err := os.Setenv("IS_CI", "true")
		require.NoError(t, err)
		defer func() {
			os.Unsetenv("IS_CI")
		}()

		err = utils.UpdateAssetsSubpathFromConfig(nil)
		require.NoError(t, err)
	})

	t.Run("no config", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "test_update_assets_subpath")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)
		currentDir, err := os.Getwd()
		require.NoError(t, err)
		os.Chdir(tempDir)
		defer os.Chdir(currentDir)

		err = utils.UpdateAssetsSubpathFromConfig(nil)
		require.Error(t, err)
	})
}

func TestUpdateAssetsSubpath(t *testing.T) {
	contentSecurityPolicyNotFoundHtml := mustReadTestFile(t, "content-security-policy-not-found.html")
	contentSecurityPolicyNotFound2Html := mustReadTestFile(t, "content-security-policy-not-found2.html")
	baseRootHtml := mustReadTestFile(t, "base-root.html")
	baseCss := mustReadTestFile(t, "base.css")
	subpathRootHtml := mustReadTestFile(t, "subpath-root.html")
	subpathCss := mustReadTestFile(t, "subpath.css")
	newSubpathRootHtml := mustReadTestFile(t, "new-subpath-root.html")
	newSubpathCss := mustReadTestFile(t, "new-subpath.css")
	baseManifestJson := mustReadTestFile(t, "base-manifest.json")
	subpathManifestJson := mustReadTestFile(t, "subpath-manifest.json")
	newSubpathManifestJson := mustReadTestFile(t, "new-subpath-manifest.json")

	t.Run("no client dir", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "test_update_assets_subpath")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)
		currentDir, err := os.Getwd()
		require.NoError(t, err)
		os.Chdir(tempDir)
		defer os.Chdir(currentDir)

		err = utils.UpdateAssetsSubpath("/")
		require.Error(t, err)
	})

	t.Run("valid", func(t *testing.T) {
		tempDir, err := ioutil.TempDir("", "test_update_assets_subpath")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)
		currentDir, err := os.Getwd()
		require.NoError(t, err)
		os.Chdir(tempDir)
		defer os.Chdir(currentDir)

		err = os.Mkdir(model.CLIENT_DIR, 0700)
		require.NoError(t, err)

		testCases := []struct {
			Description          string
			RootHTML             string
			MainCSS              string
			ManifestJSON         string
			Subpath              string
			ExpectedError        error
			ExpectedRootHTML     string
			ExpectedMainCSS      string
			ExpectedManifestJSON string
		}{
			{
				"no changes required, empty subpath provided",
				baseRootHtml,
				baseCss,
				baseManifestJson,
				"",
				nil,
				baseRootHtml,
				baseCss,
				baseManifestJson,
			},
			{
				"no changes required",
				baseRootHtml,
				baseCss,
				baseManifestJson,
				"/",
				nil,
				baseRootHtml,
				baseCss,
				baseManifestJson,
			},
			{
				"content security policy not found (missing quotes)",
				contentSecurityPolicyNotFoundHtml,
				baseCss,
				baseManifestJson,
				"/subpath",
				fmt.Errorf("failed to find 'Content-Security-Policy' meta tag to rewrite"),
				contentSecurityPolicyNotFoundHtml,
				baseCss,
				baseManifestJson,
			},
			{
				"content security policy not found (missing unsafe-eval)",
				contentSecurityPolicyNotFound2Html,
				baseCss,
				baseManifestJson,
				"/subpath",
				fmt.Errorf("failed to find 'Content-Security-Policy' meta tag to rewrite"),
				contentSecurityPolicyNotFound2Html,
				baseCss,
				baseManifestJson,
			},
			{
				"subpath",
				baseRootHtml,
				baseCss,
				baseManifestJson,
				"/subpath",
				nil,
				subpathRootHtml,
				subpathCss,
				subpathManifestJson,
			},
			{
				"new subpath from old",
				subpathRootHtml,
				subpathCss,
				subpathManifestJson,
				"/nested/subpath",
				nil,
				newSubpathRootHtml,
				newSubpathCss,
				newSubpathManifestJson,
			},
			{
				"resetting to /",
				subpathRootHtml,
				subpathCss,
				baseManifestJson,
				"/",
				nil,
				baseRootHtml,
				baseCss,
				baseManifestJson,
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.Description, func(t *testing.T) {
				ioutil.WriteFile(filepath.Join(tempDir, model.CLIENT_DIR, "root.html"), []byte(testCase.RootHTML), 0700)
				ioutil.WriteFile(filepath.Join(tempDir, model.CLIENT_DIR, "main.css"), []byte(testCase.MainCSS), 0700)
				ioutil.WriteFile(filepath.Join(tempDir, model.CLIENT_DIR, "manifest.json"), []byte(testCase.ManifestJSON), 0700)
				err := utils.UpdateAssetsSubpath(testCase.Subpath)
				if testCase.ExpectedError != nil {
					require.Equal(t, testCase.ExpectedError, err)
				} else {
					require.NoError(t, err)
				}

				contents, err := ioutil.ReadFile(filepath.Join(tempDir, model.CLIENT_DIR, "root.html"))
				require.NoError(t, err)

				// Rewrite the expected and contents for simpler diffs when failed.
				expectedRootHTML := strings.Replace(testCase.ExpectedRootHTML, ">", ">\n", -1)
				contentsStr := strings.Replace(string(contents), ">", ">\n", -1)
				require.Equal(t, expectedRootHTML, contentsStr)

				contents, err = ioutil.ReadFile(filepath.Join(tempDir, model.CLIENT_DIR, "main.css"))
				require.NoError(t, err)
				require.Equal(t, testCase.ExpectedMainCSS, string(contents))

				contents, err = ioutil.ReadFile(filepath.Join(tempDir, model.CLIENT_DIR, "manifest.json"))
				require.NoError(t, err)
				require.Equal(t, testCase.ExpectedManifestJSON, string(contents))
			})
		}
	})
}

func TestGetSubpathFromConfig(t *testing.T) {
	testCases := []struct {
		Description     string
		SiteURL         *string
		ExpectedError   bool
		ExpectedSubpath string
	}{
		{
			"empty SiteURL",
			sToP(""),
			false,
			"/",
		},
		{
			"invalid SiteURL",
			sToP("cache_object:foo/bar"),
			true,
			"",
		},
		{
			"nil SiteURL",
			nil,
			false,
			"/",
		},
		{
			"no trailing slash",
			sToP("http://localhost:8065"),
			false,
			"/",
		},
		{
			"trailing slash",
			sToP("http://localhost:8065/"),
			false,
			"/",
		},
		{
			"subpath, no trailing slash",
			sToP("http://localhost:8065/subpath"),
			false,
			"/subpath",
		},
		{
			"trailing slash",
			sToP("http://localhost:8065/subpath/"),
			false,
			"/subpath",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			config := &model.Config{
				ServiceSettings: model.ServiceSettings{
					SiteURL: testCase.SiteURL,
				},
			}

			subpath, err := utils.GetSubpathFromConfig(config)
			if testCase.ExpectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, testCase.ExpectedSubpath, subpath)
		})
	}
}

func sToP(s string) *string {
	return &s
}
